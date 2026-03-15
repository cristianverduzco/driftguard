package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/cristianverduzco/driftguard/internal/dashboard"
	"github.com/cristianverduzco/driftguard/internal/drift"
	"github.com/cristianverduzco/driftguard/internal/git"
	dgmetrics "github.com/cristianverduzco/driftguard/internal/metrics"
	"github.com/cristianverduzco/driftguard/internal/notifier"
	"github.com/cristianverduzco/driftguard/internal/remediation"
)

func main() {
	var (
		gitURL         string
		gitBranch      string
		kubeconfigPath string
		syncInterval   int
		metricsAddr    string
		dashboardAddr  string
		dryRun         bool
		autoRemediate  bool
		slackWebhook   string
	)

	flag.StringVar(&gitURL, "git-url", "", "Git repository URL to watch (required)")
	flag.StringVar(&gitBranch, "git-branch", "main", "Git branch to watch")
	flag.StringVar(&kubeconfigPath, "kubeconfig", os.Getenv("KUBECONFIG"), "Path to kubeconfig file")
	flag.IntVar(&syncInterval, "sync-interval", 30, "How often to sync in seconds")
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "Address to expose Prometheus metrics")
	flag.StringVar(&dashboardAddr, "dashboard-addr", ":9091", "Address to serve the web dashboard")
	flag.BoolVar(&dryRun, "dry-run", false, "Print what would be remediated without making changes")
	flag.BoolVar(&autoRemediate, "auto-remediate", false, "Automatically re-apply drifted manifests")
	flag.StringVar(&slackWebhook, "slack-webhook", os.Getenv("SLACK_WEBHOOK_URL"), "Slack webhook URL for drift notifications")
	flag.Parse()

	if gitURL == "" {
		fmt.Fprintln(os.Stderr, "error: --git-url is required")
		os.Exit(1)
	}

	fmt.Printf("🚀 DriftGuard starting\n")
	fmt.Printf("   Git URL:        %s\n", gitURL)
	fmt.Printf("   Branch:         %s\n", gitBranch)
	fmt.Printf("   Sync interval:  %ds\n", syncInterval)
	fmt.Printf("   Auto-remediate: %v\n", autoRemediate)
	fmt.Printf("   Dry run:        %v\n", dryRun)
	fmt.Printf("   Metrics:        %s\n", metricsAddr)
	fmt.Printf("   Dashboard:      %s\n", dashboardAddr)
	fmt.Printf("   Slack:          %v\n", slackWebhook != "")

	localPath := "/tmp/driftguard-repo"
	repo := git.NewRepo(gitURL, gitBranch, localPath)

	fmt.Println("📥 Cloning repository...")
	if err := repo.Clone(); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to clone repo: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Repository cloned")

	detector, err := drift.NewDetector(kubeconfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create detector: %v\n", err)
		os.Exit(1)
	}

	remediator, err := remediation.NewRemediator(kubeconfigPath, dryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create remediator: %v\n", err)
		os.Exit(1)
	}

	slack := notifier.NewSlackNotifier(slackWebhook)
	dash := dashboard.NewServer(dashboardAddr, gitURL)

	// Start Prometheus metrics server
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		})
		fmt.Printf("📊 Metrics server listening on %s\n", metricsAddr)
		if err := http.ListenAndServe(metricsAddr, nil); err != nil {
			fmt.Fprintf(os.Stderr, "metrics server error: %v\n", err)
		}
	}()

	// Start dashboard server
	go func() {
		if err := dash.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "dashboard server error: %v\n", err)
		}
	}()

	ticker := time.NewTicker(time.Duration(syncInterval) * time.Second)
	defer ticker.Stop()

	runSync(repo, detector, remediator, slack, dash, autoRemediate)

	for range ticker.C {
		runSync(repo, detector, remediator, slack, dash, autoRemediate)
	}
}

func runSync(
	repo *git.Repo,
	detector *drift.Detector,
	remediator *remediation.Remediator,
	slack *notifier.SlackNotifier,
	dash *dashboard.Server,
	autoRemediate bool,
) {
	ctx := context.Background()
	timer := prometheus.NewTimer(dgmetrics.SyncDuration)
	start := time.Now()
	defer timer.ObserveDuration()

	fmt.Printf("\n🔄 Starting sync at %s\n", time.Now().Format(time.RFC3339))

	if err := repo.Pull(); err != nil {
		fmt.Printf("⚠ Git pull failed: %v\n", err)
		dgmetrics.GitPullTotal.WithLabelValues("failure").Inc()
		return
	}
	dgmetrics.GitPullTotal.WithLabelValues("success").Inc()

	commit, _ := repo.GetCurrentCommit()
	fmt.Printf("📌 Current commit: %s\n", commit[:8])

	manifests, err := repo.GetManifests()
	if err != nil {
		fmt.Printf("⚠ Failed to get manifests: %v\n", err)
		return
	}
	fmt.Printf("📂 Found %d manifests\n", len(manifests))

	drifts, err := detector.DetectDrift(ctx, manifests)
	if err != nil {
		fmt.Printf("⚠ Drift detection failed: %v\n", err)
		return
	}

	dgmetrics.DriftDetected.Reset()

	dashDrifts := []dashboard.DriftRecord{}

	if len(drifts) == 0 {
		fmt.Println("✅ No drift detected — cluster matches desired state")
		if err := slack.NotifyResolved(commit); err != nil {
			fmt.Printf("⚠ Slack notification failed: %v\n", err)
		}
	} else {
		fmt.Printf("⚠ Detected %d drifted resource(s):\n", len(drifts))
		events := []notifier.DriftEvent{}
		for _, d := range drifts {
			fmt.Printf("  - %s/%s (namespace: %s, reason: %s)\n", d.Kind, d.Name, d.Namespace, d.Reason)
			dgmetrics.DriftDetected.WithLabelValues(d.Kind, d.Namespace, d.Name).Set(1)
			dgmetrics.DriftTotal.WithLabelValues(d.Kind, d.Namespace).Inc()

			remediated := false
			if autoRemediate {
				fmt.Printf("  🔧 Remediating %s/%s...\n", d.Kind, d.Name)
				for _, path := range manifests {
					if err := remediator.RemediateManifest(ctx, path); err != nil {
						fmt.Printf("  ✗ Remediation failed: %v\n", err)
						dgmetrics.RemediationTotal.WithLabelValues(d.Kind, d.Namespace, "failure").Inc()
					} else {
						remediated = true
						dgmetrics.RemediationTotal.WithLabelValues(d.Kind, d.Namespace, "success").Inc()
					}
				}
			}

			events = append(events, notifier.DriftEvent{
				Kind:      d.Kind,
				Name:      d.Name,
				Namespace: d.Namespace,
				Reason:    d.Reason,
				Commit:    commit,
			})

			dashDrifts = append(dashDrifts, dashboard.DriftRecord{
				Timestamp:  time.Now(),
				Kind:       d.Kind,
				Name:       d.Name,
				Namespace:  d.Namespace,
				Reason:     d.Reason,
				Remediated: remediated,
				Commit:     commit,
			})
		}
		if err := slack.NotifyDrift(events, commit); err != nil {
			fmt.Printf("⚠ Slack notification failed: %v\n", err)
		}
	}

	dash.RecordSync(commit, dashDrifts, time.Since(start))
	dgmetrics.LastSyncTimestamp.SetToCurrentTime()
	fmt.Printf("✓ Sync complete\n")
}