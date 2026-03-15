# DriftGuard

> A lightweight Kubernetes infrastructure drift detector and auto-remediator that continuously compares your live cluster state against a Git repository and alerts or self-heals when divergence is detected.

DriftGuard implements the core principle behind GitOps: **your Git repository is the source of truth**. When someone manually modifies a resource, deletes a Deployment, or the cluster drifts from the declared state for any reason, DriftGuard detects it immediately and can automatically re-apply the correct state from Git.

## How It Works
```
Every N seconds:
  → Pull latest manifests from Git repository
  → Fetch live state from Kubernetes API
  → Diff desired state (Git) vs actual state (cluster)
  → If drift detected:
      → Emit Prometheus metrics
      → Log detailed drift report
      → Optionally re-apply manifests to remediate
```

## Features

- **Continuous Drift Detection** — polls your Git repo and compares against live cluster state on a configurable interval
- **Auto-Remediation** — optionally re-applies drifted manifests using Kubernetes server-side apply
- **Dry Run Mode** — see what would be remediated without making any changes
- **Prometheus Metrics** — exposes drift counts, remediation totals, sync duration histograms, and last sync timestamp
- **Multi-Resource Support** — detects drift across Deployments, Services, ConfigMaps, StatefulSets, ClusterRoles, and more
- **Git-Native** — uses go-git for pure Go Git operations with no dependency on the git binary
- **In-cluster & Local** — runs inside Kubernetes or locally with a kubeconfig

## Architecture
```
┌─────────────────────────────────────────────────────┐
│                    DriftGuard                        │
│                                                     │
│   ┌─────────┐    ┌──────────┐    ┌──────────────┐  │
│   │   Git   │    │  Drift   │    │ Remediator   │  │
│   │ Watcher │───▶│ Detector │───▶│ (optional)   │  │
│   └─────────┘    └──────────┘    └──────────────┘  │
│        │               │                │           │
│        ▼               ▼                ▼           │
│   ┌─────────┐    ┌──────────┐    ┌──────────────┐  │
│   │  Git    │    │Kubernetes│    │  Prometheus  │  │
│   │  Repo   │    │   API    │    │   Metrics    │  │
│   └─────────┘    └──────────┘    └──────────────┘  │
└─────────────────────────────────────────────────────┘
```

## Metrics

DriftGuard exposes the following Prometheus metrics at `:8080/metrics`:

| Metric | Type | Description |
|---|---|---|
| `driftguard_drift_detected` | Gauge | Resources currently drifted, labeled by kind/namespace/name |
| `driftguard_drift_total` | Counter | Total drift events detected since startup |
| `driftguard_remediation_total` | Counter | Total remediations performed, labeled by status |
| `driftguard_sync_duration_seconds` | Histogram | Duration of each sync loop |
| `driftguard_last_sync_timestamp_seconds` | Gauge | Unix timestamp of last completed sync |
| `driftguard_git_pull_total` | Counter | Total git pull operations, labeled by status |

## Installation

### Run Locally
```bash
# Clone DriftGuard
git clone https://github.com/cristianverduzco/driftguard
cd driftguard

# Build
go build -o bin/driftguard ./cmd/driftguard

# Run against a Git repo
./bin/driftguard \
  --git-url https://github.com/yourorg/your-infra-repo \
  --git-branch main \
  --kubeconfig ~/.kube/config \
  --sync-interval 30 \
  --metrics-addr :8080
```

### Run with Auto-Remediation
```bash
./bin/driftguard \
  --git-url https://github.com/yourorg/your-infra-repo \
  --git-branch main \
  --kubeconfig ~/.kube/config \
  --sync-interval 30 \
  --auto-remediate
```

### Dry Run Mode
```bash
./bin/driftguard \
  --git-url https://github.com/yourorg/your-infra-repo \
  --git-branch main \
  --kubeconfig ~/.kube/config \
  --dry-run
```

### Deploy to Kubernetes
```bash
# Coming soon: Helm chart
kubectl apply -f config/deploy/driftguard.yaml
```

## CLI Flags

| Flag | Default | Description |
|---|---|---|
| `--git-url` | required | Git repository URL to watch |
| `--git-branch` | `main` | Git branch to track |
| `--kubeconfig` | `$KUBECONFIG` | Path to kubeconfig (uses in-cluster config if unset) |
| `--sync-interval` | `30` | Seconds between sync loops |
| `--metrics-addr` | `:8080` | Address to expose Prometheus metrics |
| `--auto-remediate` | `false` | Automatically re-apply drifted manifests |
| `--dry-run` | `false` | Print what would be remediated without making changes |

## Example Output
```
🚀 DriftGuard starting
   Git URL:        https://github.com/yourorg/infra
   Branch:         main
   Sync interval:  30s
   Auto-remediate: true
   Dry run:        false
   Metrics:        :8080

📥 Cloning repository...
✓ Repository cloned
📊 Metrics server listening on :8080

🔄 Starting sync at 2026-03-15T00:00:00Z
📌 Current commit: a3f2c1b8
📂 Found 12 manifests
⚠ Detected 1 drifted resource(s):
  - Deployment/my-postgres (namespace: default, reason: missing)
  🔧 Remediating Deployment/my-postgres...
  ✓ Remediated Deployment/my-postgres in namespace default
✓ Sync complete

🔄 Starting sync at 2026-03-15T00:00:30Z
📌 Current commit: a3f2c1b8
📂 Found 12 manifests
✅ No drift detected — cluster matches desired state
✓ Sync complete
```

## Supported Resource Types

- Deployments, StatefulSets, DaemonSets
- Services, Ingresses
- ConfigMaps, Secrets
- ServiceAccounts, ClusterRoles, ClusterRoleBindings
- Namespaces
- PersistentVolumeClaims
- CustomResourceDefinitions

## Stack

| Layer | Technology |
|---|---|
| Language | Go |
| Git integration | go-git (pure Go) |
| Kubernetes client | client-go (dynamic client) |
| Observability | Prometheus |
| Deployment | Docker, Kubernetes |

## Roadmap

- [ ] Helm chart for in-cluster deployment
- [ ] Slack / webhook notifications on drift detection
- [ ] Web dashboard showing drift history
- [ ] Support for private Git repositories (SSH keys, tokens)
- [ ] Ignore rules — exclude certain resources from drift detection
- [ ] Multi-cluster support

## Status

🚧 Under active development