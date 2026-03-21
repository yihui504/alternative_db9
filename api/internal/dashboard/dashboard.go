package dashboard

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.Engine) {
	r.GET("/", func(c *gin.Context) {
		c.Header("Content-Type", "text/html")
		html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>OpenClaw-db9 Dashboard</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #0f172a; color: #e2e8f0; min-height: 100vh; }
        .container { max-width: 1400px; margin: 0 auto; padding: 20px; }
        header { background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%); padding: 20px; border-radius: 12px; margin-bottom: 24px; }
        header h1 { font-size: 28px; font-weight: 700; }
        header p { opacity: 0.9; margin-top: 4px; }
        .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
        .card { background: #1e293b; border-radius: 12px; padding: 20px; border: 1px solid #334155; }
        .card h2 { font-size: 16px; color: #94a3b8; margin-bottom: 12px; text-transform: uppercase; letter-spacing: 0.05em; }
        .stat { font-size: 36px; font-weight: 700; color: #f1f5f9; }
        .stat-label { font-size: 14px; color: #64748b; margin-top: 4px; }
        .section { margin-top: 24px; }
        .btn { display: inline-block; padding: 10px 16px; background: #6366f1; color: white; border: none; border-radius: 8px; cursor: pointer; font-size: 14px; text-decoration: none; }
        .btn:hover { background: #4f46e5; }
        .nav { display: flex; gap: 8px; margin-bottom: 16px; flex-wrap: wrap; }
        .nav-btn { padding: 8px 16px; background: #334155; border: none; border-radius: 6px; color: #e2e8f0; cursor: pointer; }
        .nav-btn.active { background: #6366f1; }
        .panel { display: none; }
        .panel.active { display: block; }
        #sql-editor { width: 100%; height: 120px; background: #0f172a; border: 1px solid #334155; border-radius: 8px; color: #e2e8f0; padding: 12px; font-family: 'Monaco', 'Menlo', monospace; resize: vertical; }
        #sql-result { background: #0f172a; border-radius: 8px; padding: 12px; margin-top: 12px; max-height: 300px; overflow: auto; font-family: 'Monaco', 'Menlo', monospace; font-size: 13px; white-space: pre-wrap; }
        .success { color: #22c55e; }
        .error { color: #ef4444; }
        table { width: 100%; border-collapse: collapse; margin-top: 12px; }
        th, td { text-align: left; padding: 12px; border-bottom: 1px solid #334155; }
        th { color: #94a3b8; font-weight: 500; font-size: 12px; text-transform: uppercase; }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>OpenClaw-db9 Dashboard</h1>
            <p>v1.1.0 | Database API for AI Agents</p>
        </header>

        <div class="grid">
            <div class="card">
                <h2>Databases</h2>
                <div class="stat" id="db-count">-</div>
                <div class="stat-label">Total databases</div>
            </div>
            <div class="card">
                <h2>Branches</h2>
                <div class="stat" id="branch-count">-</div>
                <div class="stat-label">Total branches</div>
            </div>
            <div class="card">
                <h2>Health</h2>
                <div class="stat" id="health-status">-</div>
                <div class="stat-label" id="health-detail">Checking...</div>
            </div>
        </div>

        <div class="section">
            <div class="nav">
                <button class="nav-btn active" onclick="showPanel('sql')">SQL Query</button>
                <button class="nav-btn" onclick="showPanel('api')">API Reference</button>
                <button class="nav-btn" onclick="showPanel('databases')">Databases</button>
            </div>

            <div id="panel-sql" class="panel active">
                <div class="card">
                    <h2>Execute SQL</h2>
                    <select id="db-selector" style="width: 100%; padding: 10px; margin-bottom: 12px; background: #1e293b; color: #e2e8f0; border: 1px solid #334155; border-radius: 6px;">
                        <option value="">Select a database...</option>
                    </select>
                    <textarea id="sql-editor" placeholder="Enter SQL query...">SELECT 1 as test</textarea>
                    <div style="margin-top: 12px;">
                        <button class="btn" onclick="executeSQL()">Execute</button>
                    </div>
                    <div id="sql-result"></div>
                </div>
            </div>

            <div id="panel-api" class="panel">
                <div class="card">
                    <h2>API Reference</h2>
                    <p style="color: #94a3b8;">Visit <a href="/api/docs" style="color: #6366f1;">/api/docs</a> for complete API documentation.</p>
                </div>
            </div>

            <div id="panel-databases" class="panel">
                <div class="card">
                    <h2>Databases</h2>
                    <button class="btn" onclick="loadDatabases()">Refresh</button>
                    <div id="databases-list"></div>
                </div>
            </div>
        </div>
    </div>

    <script>
        const API_BASE = '/api/v1';

        async function loadHealth() {
            try {
                const res = await fetch('/health');
                const data = await res.json();
                document.getElementById('health-status').textContent = data.status.toUpperCase();
                document.getElementById('health-status').style.color = data.status === 'healthy' ? '#22c55e' : '#f59e0b';
                const components = Object.entries(data.components || {}).map(([k, v]) => k + ': ' + v).join(', ');
                document.getElementById('health-detail').textContent = components || 'All systems operational';
            } catch (e) {
                document.getElementById('health-status').textContent = 'ERROR';
                document.getElementById('health-status').style.color = '#ef4444';
            }
        }

        async function loadStats() {
            try {
                const res = await fetch(API_BASE + '/monitor/stats');
                const data = await res.json();
                document.getElementById('db-count').textContent = data.total_databases || 0;
                document.getElementById('branch-count').textContent = data.total_branches || 0;
            } catch (e) { console.error(e); }
        }

        async function loadDatabases() {
            try {
                const res = await fetch(API_BASE + '/databases');
                const databases = await res.json();
                const selector = document.getElementById('db-selector');
                selector.innerHTML = '<option value="">Select a database...</option>' +
                    databases.map(db => '<option value="' + db.id + '">' + db.name + '</option>').join('');
                const list = document.getElementById('databases-list');
                list.innerHTML = databases.length ? '<table><tr><th>Name</th><th>ID</th><th>Created</th></tr>' +
                    databases.map(db => '<tr><td>' + db.name + '</td><td style="font-size:12px;color:#94a3b8">' + db.id + '</td><td>' + new Date(db.created_at).toLocaleString() + '</td></tr>').join('') + '</table>'
                    : '<p style="color:#94a3b8">No databases found.</p>';
            } catch (e) { document.getElementById('databases-list').innerHTML = '<p class="error">Failed to load</p>'; }
        }

        async function executeSQL() {
            const dbId = document.getElementById('db-selector').value;
            const sql = document.getElementById('sql-editor').value;
            const resultDiv = document.getElementById('sql-result');
            if (!dbId) { resultDiv.innerHTML = '<span class="error">Select a database first</span>'; return; }
            resultDiv.innerHTML = 'Executing...';
            try {
                const res = await fetch(API_BASE + '/databases/' + dbId + '/sql', {
                    method: 'POST', headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ sql })
                });
                const data = await res.json();
                if (res.ok) {
                    resultDiv.innerHTML = data.results
                        ? '<span class="success">Success (' + data.results.length + ' rows)</span>\n' + JSON.stringify(data.results, null, 2)
                        : '<span class="success">' + (data.message || 'Done') + '</span>';
                } else {
                    resultDiv.innerHTML = '<span class="error">Error: ' + data.error + '</span>';
                }
            } catch (e) { resultDiv.innerHTML = '<span class="error">Request failed: ' + e.message + '</span>'; }
        }

        function showPanel(name) {
            document.querySelectorAll('.panel').forEach(p => p.classList.remove('active'));
            document.querySelectorAll('.nav-btn').forEach(b => b.classList.remove('active'));
            document.getElementById('panel-' + name).classList.add('active');
            event.target.classList.add('active');
        }

        loadHealth(); loadStats(); loadDatabases();
        setInterval(loadHealth, 30000);
        setInterval(loadStats, 30000);
    </script>
</body>
</html>`
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	})
}
