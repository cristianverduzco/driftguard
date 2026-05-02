# DriftGuard

> A Kubernetes infrastructure drift detector and auto-remediator written in Go that continuously compares live cluster state against a Git repository and self-heals when divergence is detected.

DriftGuard implements the core principle behind GitOps: **your Git repository is the source of truth**. When someone manually deletes a Deployment, modifies a resource outside of Git, or the cluster drifts for any reason, DriftGuard detects it within seconds, fires a Slack alert, auto-remediates the drift, and records the event in a real-time web dashboard.

Built from scratch to understand how production GitOps tools like Argo CD and Flux work under the hood.

---

## How It Works
```
Every N seconds:
  → Pull latest commit from Git repository (go-git, shallow clone)
  → Parse all YAML manifests in the repo
  → Query Kubernetes API for each resource (dynamic client)
  → Diff desired state (Git) vs actual state (cluster)
  → If drift detected:
      → Emit Prometheus metrics
      → Send Slack alert
      → Optionally re-apply via server-side apply
      → Record event in web dashboard
```

---

## Features

- **Continuous Drift Detection** — polls your Git repo and compares against live cluster state on a configurable interval
- **Auto-Remediation** — re-applies drifted manifests using Kubernetes server-side apply with field manager tracking
- **Slack Notifications** — sends rich drift alerts and resolved notifications via webhook
- **Real-Time Dashboard** — web UI at `:9091` showing live sync history, drift events, and remediation status
- **Prometheus Metrics** — drift counts, remediation totals, sync duration histograms, and last sync timestamp
- **Dry Run Mode** — see exactly what would be remediated without making any changes
- **Git-Native** — uses go-git for pure Go Git operations with no dependency on the git binary
- **Distroless Container** — minimal attack surface, runs as non-root, ~8MB image
- **Helm Chart** — production-ready chart with RBAC, optional Slack Secret, and ServiceMonitor
- **In-cluster & Local** — runs inside Kubernetes or locally with a kubeconfig

---

## Architecture
```
┌──────────────────────────────────────────────────────────────┐
│                         DriftGuard                           │
│                                                              │
│  ┌──────────┐   ┌──────────┐   ┌────────────┐   ┌────────┐ │
│  │   Git    │   │  Drift   │   │ Remediator │   │ Slack  │ │
│  │ Watcher  │──▶│ Detector │──▶│ (SSA)      │──▶│ Alerts │ │
│  └──────────┘   └──────────┘   └────────────┘   └────────┘ │
│       │               │                │                     │
│       ▼               ▼                ▼                     │
│  ┌──────────┐   ┌──────────┐   ┌────────────┐              │
│  │   Git    │   │   K8s    │   │ Prometheus │              │
│  │   Repo   │   │   API    │   │  Metrics   │              │
│  └──────────┘   └──────────┘   └────────────┘              │
│                                                              │
│                    ┌─────────────────┐                      │
│                    │  Web Dashboard  │ :9091                │
│                    └─────────────────┘                      │
└──────────────────────────────────────────────────────────────┘
```

---

## Demo

**Drift detected and auto-remediated:**
```
🔄 Starting sync at 2026-03-15T20:51:07Z
📌 Current commit: 2c9f0525
📂 Found 1 manifests
⚠ Detected 1 drifted resource(s):
  - Deployment/driftguard-test (namespace: default, reason: missing)
  🔧 Remediating Deployment/driftguard-test...
  ✓ Remediated Deployment/driftguard-test in namespace default
✓ Sync complete

🔄 Starting sync at 2026-03-15T20:51:37Z
📌 Current commit: 2c9f0525
📂 Found 1 manifests
✅ No drift detected — cluster matches desired state
✓ Sync complete
```

---

## Prometheus Metrics

Exposed at `:8080/metrics`:

| Metric | Type | Description |
|---|---|---|
| `driftguard_drift_detected` | Gauge | Resources currently drifted, labeled by kind/namespace/name |
| `driftguard_drift_total` | Counter | Total drift events detected since startup |
| `driftguard_remediation_total` | Counter | Total remediations by success/failure status |
| `driftguard_sync_duration_seconds` | Histogram | Duration of each sync loop |
| `driftguard_last_sync_timestamp_seconds` | Gauge | Unix timestamp of last completed sync |
| `driftguard_git_pull_total` | Counter | Git pull operations by success/failure |

---

## Installation

### Run Locally
```bash
git clone https://github.com/cristianverduzco/driftguard
cd driftguard
go build -o bin/driftguard ./cmd/driftguard

./bin/driftguard \
  --git-url https://github.com/yourorg/infra \
  --git-branch main \
  --kubeconfig ~/.kube/config \
  --sync-interval 30
```

### Run with Auto-Remediation
```bash
./bin/driftguard \
  --git-url https://github.com/yourorg/infra \
  --kubeconfig ~/.kube/config \
  --auto-remediate \
  --slack-webhook https://hooks.slack.com/services/...
```

### Deploy to Kubernetes via Helm
```bash
helm install driftguard ./charts/driftguard \
  --set git.url=https://github.com/yourorg/infra \
  --set autoRemediate=true \
  --set slackWebhookUrl=https://hooks.slack.com/services/...
```

---

## CLI Flags

| Flag | Default | Description |
|---|---|---|
| `--git-url` | required | Git repository URL to watch |
| `--git-branch` | `main` | Git branch to track |
| `--kubeconfig` | `$KUBECONFIG` | Path to kubeconfig (uses in-cluster config if unset) |
| `--sync-interval` | `30` | Seconds between sync loops |
| `--metrics-addr` | `:8080` | Address to expose Prometheus metrics |
| `--dashboard-addr` | `:9091` | Address to serve the web dashboard |
| `--auto-remediate` | `false` | Automatically re-apply drifted manifests |
| `--dry-run` | `false` | Print what would be remediated without making changes |
| `--slack-webhook` | `$SLACK_WEBHOOK_URL` | Slack webhook URL for drift notifications |

---

## Supported Resource Types

Deployments, StatefulSets, DaemonSets, Services, Ingresses, ConfigMaps, Secrets, ServiceAccounts, ClusterRoles, ClusterRoleBindings, Namespaces, PersistentVolumeClaims, CustomResourceDefinitions

---

## Stack

| Layer | Technology |
|---|---|
| Language | Go |
| Git integration | go-git (pure Go, no git binary) |
| Kubernetes client | client-go dynamic client |
| Remediation | Kubernetes server-side apply |
| Observability | Prometheus |
| Notifications | Slack incoming webhooks |
| Packaging | Helm, distroless Docker |
| Infrastructure | Kubernetes (kubeadm), Arch Linux |

---

## Roadmap

**Implemented:**
- ✅ Continuous drift detection against Git desired state
- ✅ Auto-remediation via server-side apply with field manager tracking
- ✅ Slack notifications on drift and resolved events
- ✅ Real-time web dashboard with sync and drift history
- ✅ Prometheus metrics with histograms and counters
- ✅ Helm chart with RBAC and ServiceMonitor
- ✅ Distroless container, runs as non-root
- ✅ Deployed on self-hosted kubeadm cluster

**Planned:**
- Field-level diff detection (currently resource-existence only)
- Configurable ignore rules for Kubernetes-managed fields
- Private Git repository support (SSH keys, deploy tokens)
- Multi-cluster support
- Persistent drift history with database backend

---

## Status

✅ Core features complete and running on a self-hosted kubeadm cluster (Arch Linux, Kubernetes v1.35)
