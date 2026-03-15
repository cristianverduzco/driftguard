package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// DriftDetected tracks the number of drifted resources by kind and namespace
	DriftDetected = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "driftguard_drift_detected",
			Help: "Number of resources currently drifted from desired state",
		},
		[]string{"kind", "namespace", "name"},
	)

	// DriftTotal tracks the total number of drift events detected since startup
	DriftTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "driftguard_drift_total",
			Help: "Total number of drift events detected",
		},
		[]string{"kind", "namespace"},
	)

	// RemediationTotal tracks the total number of remediations performed
	RemediationTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "driftguard_remediation_total",
			Help: "Total number of remediations performed",
		},
		[]string{"kind", "namespace", "status"},
	)

	// SyncDuration tracks how long each sync loop takes
	SyncDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "driftguard_sync_duration_seconds",
			Help:    "Duration of each drift detection sync loop",
			Buckets: prometheus.DefBuckets,
		},
	)

	// LastSyncTimestamp tracks when the last sync completed
	LastSyncTimestamp = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "driftguard_last_sync_timestamp_seconds",
			Help: "Unix timestamp of the last completed sync",
		},
	)

	// GitPullTotal tracks git pull operations
	GitPullTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "driftguard_git_pull_total",
			Help: "Total number of git pull operations",
		},
		[]string{"status"},
	)
)

func init() {
	prometheus.MustRegister(SyncDuration)
}