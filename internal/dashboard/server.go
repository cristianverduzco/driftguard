package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type DriftRecord struct {
	Timestamp  time.Time `json:"timestamp"`
	Kind       string    `json:"kind"`
	Name       string    `json:"name"`
	Namespace  string    `json:"namespace"`
	Reason     string    `json:"reason"`
	Remediated bool      `json:"remediated"`
	Commit     string    `json:"commit"`
}

type SyncRecord struct {
	Timestamp  time.Time     `json:"timestamp"`
	Commit     string        `json:"commit"`
	DriftCount int           `json:"driftCount"`
	Drifts     []DriftRecord `json:"drifts"`
	Duration   time.Duration `json:"duration"`
}

type DashboardState struct {
	mu           sync.RWMutex
	SyncHistory  []SyncRecord  `json:"syncHistory"`
	DriftHistory []DriftRecord `json:"driftHistory"`
	LastSync     time.Time     `json:"lastSync"`
	LastCommit   string        `json:"lastCommit"`
	TotalDrifts  int           `json:"totalDrifts"`
	TotalSyncs   int           `json:"totalSyncs"`
	GitURL       string        `json:"gitUrl"`
	IsHealthy    bool          `json:"isHealthy"`
}

type Server struct {
	state *DashboardState
	addr  string
}

func NewServer(addr string, gitURL string) *Server {
	return &Server{
		addr: addr,
		state: &DashboardState{
			SyncHistory:  []SyncRecord{},
			DriftHistory: []DriftRecord{},
			GitURL:       gitURL,
			IsHealthy:    true,
		},
	}
}

func (s *Server) RecordSync(commit string, drifts []DriftRecord, duration time.Duration) {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	record := SyncRecord{
		Timestamp:  time.Now(),
		Commit:     commit,
		DriftCount: len(drifts),
		Drifts:     drifts,
		Duration:   duration,
	}

	s.state.SyncHistory = append([]SyncRecord{record}, s.state.SyncHistory...)
	if len(s.state.SyncHistory) > 50 {
		s.state.SyncHistory = s.state.SyncHistory[:50]
	}

	for _, d := range drifts {
		s.state.DriftHistory = append([]DriftRecord{d}, s.state.DriftHistory...)
		s.state.TotalDrifts++
	}
	if len(s.state.DriftHistory) > 100 {
		s.state.DriftHistory = s.state.DriftHistory[:100]
	}

	s.state.LastSync = time.Now()
	s.state.LastCommit = commit
	s.state.TotalSyncs++
	s.state.IsHealthy = true
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/", s.handleDashboard)
	fmt.Printf("🖥️  Dashboard server listening on %s\n", s.addr)
	return http.ListenAndServe(s.addr, mux)
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(s.state)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(s.state.SyncHistory)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(getDashboardHTML()))
}

func getDashboardHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>DriftGuard Dashboard</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: #0f1117; color: #e2e8f0; min-height: 100vh; }
    .header { background: #1a1d2e; border-bottom: 1px solid #2d3748; padding: 16px 32px; display: flex; align-items: center; gap: 12px; }
    .header h1 { font-size: 20px; font-weight: 700; color: #fff; }
    .badge { background: #3b82f6; color: #fff; font-size: 11px; padding: 2px 8px; border-radius: 999px; font-weight: 600; }
    .main { padding: 32px; max-width: 1200px; margin: 0 auto; }
    .stats { display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px; margin-bottom: 32px; }
    .stat-card { background: #1a1d2e; border: 1px solid #2d3748; border-radius: 12px; padding: 20px; }
    .stat-card .label { font-size: 12px; color: #64748b; text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 8px; }
    .stat-card .value { font-size: 28px; font-weight: 700; color: #fff; }
    .value.green { color: #10b981; }
    .value.red { color: #ef4444; }
    .value.blue { color: #3b82f6; }
    .section { background: #1a1d2e; border: 1px solid #2d3748; border-radius: 12px; margin-bottom: 24px; overflow: hidden; }
    .section-header { padding: 16px 20px; border-bottom: 1px solid #2d3748; font-weight: 600; font-size: 14px; color: #94a3b8; text-transform: uppercase; letter-spacing: 0.05em; }
    table { width: 100%; border-collapse: collapse; }
    th { padding: 12px 20px; text-align: left; font-size: 11px; color: #64748b; text-transform: uppercase; letter-spacing: 0.05em; border-bottom: 1px solid #2d3748; }
    td { padding: 12px 20px; font-size: 13px; border-bottom: 1px solid #1e2433; }
    tr:last-child td { border-bottom: none; }
    tr:hover td { background: #1e2433; }
    .badge-green { background: #064e3b; color: #10b981; padding: 2px 8px; border-radius: 4px; font-size: 11px; font-weight: 600; }
    .badge-red { background: #450a0a; color: #ef4444; padding: 2px 8px; border-radius: 4px; font-size: 11px; font-weight: 600; }
    .badge-blue { background: #1e3a5f; color: #3b82f6; padding: 2px 8px; border-radius: 4px; font-size: 11px; font-weight: 600; }
    .mono { font-family: monospace; font-size: 12px; color: #94a3b8; }
    .empty { padding: 40px; text-align: center; color: #475569; font-size: 14px; }
    .git-url { font-size: 13px; color: #64748b; margin-left: auto; }
  </style>
</head>
<body>
  <div class="header">
    <span style="font-size:24px">&#x1F6E1;&#xFE0F;</span>
    <h1>DriftGuard</h1>
    <span class="badge">LIVE</span>
    <span class="git-url" id="gitUrl"></span>
  </div>
  <div class="main">
    <div class="stats">
      <div class="stat-card"><div class="label">Status</div><div class="value green" id="status">-</div></div>
      <div class="stat-card"><div class="label">Total Syncs</div><div class="value blue" id="totalSyncs">0</div></div>
      <div class="stat-card"><div class="label">Total Drifts</div><div class="value red" id="totalDrifts">0</div></div>
      <div class="stat-card"><div class="label">Last Commit</div><div class="value mono" id="lastCommit">-</div></div>
    </div>
    <div class="section">
      <div class="section-header">Sync History</div>
      <table>
        <thead><tr><th>Time</th><th>Commit</th><th>Drift Count</th><th>Status</th></tr></thead>
        <tbody id="syncHistory"><tr><td colspan="4" class="empty">Loading...</td></tr></tbody>
      </table>
    </div>
    <div class="section">
      <div class="section-header">Drift History</div>
      <table>
        <thead><tr><th>Time</th><th>Resource</th><th>Namespace</th><th>Reason</th><th>Remediated</th></tr></thead>
        <tbody id="driftHistory"><tr><td colspan="5" class="empty">No drift detected yet</td></tr></tbody>
      </table>
    </div>
  </div>
  <script>
    function timeAgo(date) {
      const s = Math.floor((new Date() - new Date(date)) / 1000);
      if (s < 60) return s + 's ago';
      if (s < 3600) return Math.floor(s/60) + 'm ago';
      return Math.floor(s/3600) + 'h ago';
    }
    async function refresh() {
      try {
        const res = await fetch('/api/state');
        const state = await res.json();
        document.getElementById('gitUrl').textContent = state.gitUrl || '';
        document.getElementById('totalSyncs').textContent = state.totalSyncs || 0;
        document.getElementById('totalDrifts').textContent = state.totalDrifts || 0;
        document.getElementById('lastCommit').textContent = (state.lastCommit || '-').substring(0,8);
        document.getElementById('status').textContent = state.isHealthy ? 'Healthy' : 'Degraded';
        document.getElementById('status').className = 'value ' + (state.isHealthy ? 'green' : 'red');
        const sb = document.getElementById('syncHistory');
        if (!state.syncHistory || state.syncHistory.length === 0) {
          sb.innerHTML = '<tr><td colspan="4" class="empty">No syncs yet</td></tr>';
        } else {
          sb.innerHTML = state.syncHistory.map(s => {
            const st = s.driftCount === 0 ? '<span class="badge-green">In Sync</span>' : '<span class="badge-red">Drifted (' + s.driftCount + ')</span>';
            return '<tr><td class="mono">' + timeAgo(s.timestamp) + '</td><td class="mono">' + (s.commit||'').substring(0,8) + '</td><td>' + s.driftCount + '</td><td>' + st + '</td></tr>';
          }).join('');
        }
        const db = document.getElementById('driftHistory');
        if (!state.driftHistory || state.driftHistory.length === 0) {
          db.innerHTML = '<tr><td colspan="5" class="empty">No drift detected yet</td></tr>';
        } else {
          db.innerHTML = state.driftHistory.map(d => {
            const rem = d.remediated ? '<span class="badge-green">Yes</span>' : '<span class="badge-blue">No</span>';
            return '<tr><td class="mono">' + timeAgo(d.timestamp) + '</td><td><strong>' + d.kind + '/' + d.name + '</strong></td><td class="mono">' + (d.namespace||'-') + '</td><td><span class="badge-red">' + d.reason + '</span></td><td>' + rem + '</td></tr>';
          }).join('');
        }
      } catch(e) { console.error(e); }
    }
    refresh();
    setInterval(refresh, 5000);
  </script>
</body>
</html>`
}
