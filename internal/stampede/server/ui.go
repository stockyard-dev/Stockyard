package server

const adminHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Stockyard Stampede</title>
<style>
:root {
  --bg: #1a1510; --bg2: #2a2318; --bg3: #3a3228;
  --fg: #f5f0e8; --fg2: #8a7e6e; --fg3: #5a5248;
  --rust: #8b4513; --cream: #d4a574; --gold: #f5c542;
  --green: #66bb6a; --red: #ef5350; --orange: #ffa726; --blue: #42a5f5;
}
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: Georgia, serif; background: var(--bg); color: var(--fg); min-height: 100vh; }
a { color: var(--cream); text-decoration: none; }
a:hover { text-decoration: underline; }

.header { background: var(--bg2); border-bottom: 2px solid var(--rust); padding: 1rem 2rem; display: flex; align-items: center; gap: 1rem; }
.header h1 { font-family: 'Libre Baskerville', Georgia, serif; font-size: 1.4rem; color: var(--cream); }
.header .badge { background: var(--rust); color: var(--fg); font-size: 0.7rem; padding: 0.2rem 0.5rem; border-radius: 3px; font-family: monospace; }

.container { max-width: 1100px; margin: 0 auto; padding: 2rem; }

.cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 1rem; margin: 1.5rem 0; }
.card { background: var(--bg2); border-radius: 8px; padding: 1.2rem; border: 1px solid var(--bg3); }
.card-value { font-size: 2rem; font-weight: bold; color: var(--cream); }
.card-label { font-size: 0.8rem; color: var(--fg2); margin-top: 0.3rem; }

h2 { font-family: 'Libre Baskerville', Georgia, serif; color: var(--cream); margin: 2rem 0 1rem; font-size: 1.3rem;
     border-bottom: 1px solid var(--bg3); padding-bottom: 0.5rem; }

table { width: 100%; border-collapse: collapse; margin: 1rem 0; }
th { background: var(--bg3); color: var(--fg); padding: 0.6rem 1rem; text-align: left; font-family: monospace; font-size: 0.8rem; text-transform: uppercase; }
td { padding: 0.6rem 1rem; border-bottom: 1px solid var(--bg3); font-size: 0.9rem; }
tr:hover td { background: var(--bg2); }
.good { color: var(--green); font-weight: bold; }
.warn { color: var(--orange); font-weight: bold; }
.bad { color: var(--red); font-weight: bold; }

.btn { background: var(--rust); color: var(--fg); border: none; padding: 0.6rem 1.2rem; border-radius: 4px; cursor: pointer; font-family: Georgia, serif; font-size: 0.9rem; }
.btn:hover { opacity: 0.85; }
.btn-sm { padding: 0.3rem 0.8rem; font-size: 0.8rem; }

.form-group { margin-bottom: 1rem; }
.form-group label { display: block; color: var(--fg2); font-size: 0.85rem; margin-bottom: 0.3rem; }
.form-group input, .form-group select { background: var(--bg); border: 1px solid var(--bg3); color: var(--fg); padding: 0.5rem 0.8rem; border-radius: 4px; width: 100%; font-family: monospace; font-size: 0.9rem; }
.form-row { display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; }

.safety-badge { display: inline-block; padding: 0.15rem 0.5rem; border-radius: 3px; font-size: 0.75rem; font-weight: bold; text-transform: uppercase; font-family: monospace; }
.safety-critical { background: var(--red); color: white; }
.safety-high { background: var(--orange); color: #111; }
.safety-medium { background: var(--gold); color: #111; }
.safety-low { background: var(--bg3); color: var(--fg2); }

.tab-bar { display: flex; gap: 0; margin-top: 2rem; border-bottom: 2px solid var(--bg3); }
.tab { padding: 0.6rem 1.5rem; cursor: pointer; color: var(--fg2); font-size: 0.9rem; border-bottom: 2px solid transparent; margin-bottom: -2px; }
.tab:hover { color: var(--fg); }
.tab.active { color: var(--cream); border-bottom-color: var(--cream); }
.tab-content { display: none; padding: 1rem 0; }
.tab-content.active { display: block; }

.empty { color: var(--fg3); text-align: center; padding: 3rem; font-style: italic; }
.spinner { display: inline-block; width: 1rem; height: 1rem; border: 2px solid var(--bg3); border-top-color: var(--cream); border-radius: 50%; animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
#status-msg { color: var(--fg2); font-size: 0.85rem; margin-top: 0.5rem; }
</style>
</head>
<body>

<div class="header">
  <h1>🐂 Stampede</h1>
  <span class="badge">AI Synthetic User Testing</span>
</div>

<div class="container">
  <div class="tab-bar">
    <div class="tab active" onclick="showTab('dashboard')">Dashboard</div>
    <div class="tab" onclick="showTab('new-sim')">New Simulation</div>
    <div class="tab" onclick="showTab('personas')">Personas</div>
  </div>

  <!-- Dashboard Tab -->
  <div id="tab-dashboard" class="tab-content active">
    <div class="cards" id="summary-cards">
      <div class="card"><div class="card-value" id="stat-total">—</div><div class="card-label">Total Simulations</div></div>
      <div class="card"><div class="card-value" id="stat-convos">—</div><div class="card-label">Conversations Run</div></div>
      <div class="card"><div class="card-value" id="stat-safety">—</div><div class="card-label">Safety Findings</div></div>
      <div class="card"><div class="card-value" id="stat-cost">—</div><div class="card-label">Total Cost</div></div>
    </div>

    <h2>Recent Simulations</h2>
    <table>
      <thead><tr><th>ID</th><th>Target</th><th>Users</th><th>Conversations</th><th>Cost</th><th>Status</th><th></th></tr></thead>
      <tbody id="sim-list"><tr><td colspan="7" class="empty">Loading...</td></tr></tbody>
    </table>
  </div>

  <!-- New Simulation Tab -->
  <div id="tab-new-sim" class="tab-content">
    <h2>Run a Simulation</h2>
    <div class="form-row">
      <div class="form-group">
        <label>Target URL *</label>
        <input type="text" id="f-target" placeholder="http://localhost:3000/chat" />
      </div>
      <div class="form-group">
        <label>API Format</label>
        <select id="f-format">
          <option value="auto">Auto-detect</option>
          <option value="openai">OpenAI-compatible</option>
          <option value="simple">Simple JSON</option>
        </select>
      </div>
    </div>
    <div class="form-row">
      <div class="form-group">
        <label>Synthetic Users</label>
        <input type="number" id="f-users" value="10" min="1" max="500" />
      </div>
      <div class="form-group">
        <label>Duration</label>
        <input type="text" id="f-duration" value="10m" placeholder="10m" />
      </div>
    </div>
    <div class="form-row">
      <div class="form-group">
        <label>Max Turns</label>
        <input type="number" id="f-turns" value="20" min="1" max="50" />
      </div>
      <div class="form-group">
        <label>Rate Limit (req/sec, 0=unlimited)</label>
        <input type="number" id="f-rate" value="0" min="0" />
      </div>
    </div>
    <div class="form-group">
      <label>Personas</label>
      <div id="persona-checks" style="display:flex;gap:1rem;flex-wrap:wrap;margin-top:0.3rem;"></div>
    </div>
    <div class="form-group">
      <label>Proxy URL (optional — route through Stockyard)</label>
      <input type="text" id="f-proxy" placeholder="http://localhost:8080" />
    </div>
    <button class="btn" onclick="startSimulation()">🐂 Start Stampede</button>
    <div id="status-msg"></div>
  </div>

  <!-- Personas Tab -->
  <div id="tab-personas" class="tab-content">
    <h2>Available Personas</h2>
    <div id="persona-list" class="empty">Loading...</div>
  </div>

  <!-- Simulation Detail (shown dynamically) -->
  <div id="tab-detail" class="tab-content">
    <div id="detail-content"></div>
  </div>
</div>

<script>
const API = window.location.origin + '/api';

function showTab(name) {
  document.querySelectorAll('.tab-content').forEach(t => t.classList.remove('active'));
  document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
  document.getElementById('tab-' + name).classList.add('active');
  document.querySelector('[onclick="showTab(\'' + name + '\')"]')?.classList.add('active');
}

async function fetchJSON(url) {
  const r = await fetch(url);
  return r.json();
}

async function loadDashboard() {
  const sims = await fetchJSON(API + '/simulations?limit=20');
  const list = document.getElementById('sim-list');

  if (!sims || sims.length === 0) {
    list.innerHTML = '<tr><td colspan="7" class="empty">No simulations yet. Run one from the "New Simulation" tab.</td></tr>';
    return;
  }

  let totalConvos = 0, totalCost = 0, totalSafety = 0;
  list.innerHTML = sims.map(s => {
    totalConvos += (s.conversations?.length || 0);
    totalCost += s.total_cost_cents || 0;
    totalSafety += (s.safety_findings?.length || 0);
    const convCount = s.conversations?.length || s.conversation_count || 0;
    return '<tr>' +
      '<td><a href="#" onclick="showDetail(\'' + s.id + '\')">' + s.id + '</a></td>' +
      '<td style="max-width:250px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">' + (s.target_url || '') + '</td>' +
      '<td>' + (s.user_count || 0) + '</td>' +
      '<td>' + convCount + '</td>' +
      '<td>$' + (s.total_cost_cents || 0).toFixed(4) + '</td>' +
      '<td>' + (s.status || 'complete') + '</td>' +
      '<td><a href="' + API + '/simulations/' + s.id + '/report?format=html" target="_blank" class="btn btn-sm">Report</a></td>' +
      '</tr>';
  }).join('');

  document.getElementById('stat-total').textContent = sims.length;
  document.getElementById('stat-convos').textContent = totalConvos;
  document.getElementById('stat-safety').textContent = totalSafety;
  document.getElementById('stat-cost').textContent = '$' + totalCost.toFixed(2);
}

async function showDetail(id) {
  showTab('detail');
  const el = document.getElementById('detail-content');
  el.innerHTML = '<div class="empty"><span class="spinner"></span> Loading...</div>';

  const sim = await fetchJSON(API + '/simulations/' + id);
  if (!sim || sim.error) {
    el.innerHTML = '<div class="empty">Simulation not found.</div>';
    return;
  }

  let html = '<h2>Simulation ' + sim.id + '</h2>';
  html += '<p style="color:var(--fg2)">Target: ' + sim.target_url + ' · Users: ' + sim.user_count + '</p>';

  // Summary cards
  const total = sim.conversations?.length || 0;
  const completed = sim.conversations?.filter(c => c.goal_result === 'completed').length || 0;
  const rate = total > 0 ? (completed / total * 100).toFixed(1) : '0.0';
  const rateClass = rate >= 75 ? 'good' : rate >= 50 ? 'warn' : 'bad';

  html += '<div class="cards">';
  html += '<div class="card"><div class="card-value">' + total + '</div><div class="card-label">Conversations</div></div>';
  html += '<div class="card"><div class="card-value ' + rateClass + '">' + rate + '%</div><div class="card-label">Goal Completion</div></div>';
  html += '<div class="card"><div class="card-value">$' + (sim.total_cost_cents || 0).toFixed(4) + '</div><div class="card-label">Cost</div></div>';
  html += '<div class="card"><div class="card-value">' + (sim.safety_findings?.length || 0) + '</div><div class="card-label">Safety Findings</div></div>';
  html += '</div>';

  // Persona breakdown
  if (sim.persona_breakdown) {
    html += '<h2>Results by Persona</h2><table>';
    html += '<tr><th>Persona</th><th>Total</th><th>Completed</th><th>Partial</th><th>Failed</th><th>Rate</th></tr>';
    for (const [name, ps] of Object.entries(sim.persona_breakdown)) {
      const rc = ps.completion_rate >= 75 ? 'good' : ps.completion_rate >= 50 ? 'warn' : 'bad';
      html += '<tr><td>' + name + '</td><td>' + ps.total + '</td><td>' + ps.completed + '</td><td>' + ps.partial + '</td><td>' + ps.failed + '</td><td class="' + rc + '">' + ps.completion_rate.toFixed(1) + '%</td></tr>';
    }
    html += '</table>';
  }

  // Safety findings
  if (sim.safety_findings && sim.safety_findings.length > 0) {
    html += '<h2>Safety Findings</h2><table>';
    html += '<tr><th>Severity</th><th>Type</th><th>Description</th><th>Conv</th><th>Turn</th></tr>';
    sim.safety_findings.forEach(f => {
      html += '<tr><td><span class="safety-badge safety-' + f.severity + '">' + f.severity + '</span></td>';
      html += '<td>' + f.type + '</td><td>' + f.description + '</td>';
      html += '<td>' + f.conversation_id + '</td><td>' + f.turn_number + '</td></tr>';
    });
    html += '</table>';
  }

  // Failure patterns
  if (sim.failure_patterns && sim.failure_patterns.length > 0) {
    html += '<h2>Failure Patterns</h2><table>';
    html += '<tr><th>#</th><th>Pattern</th><th>Count</th></tr>';
    sim.failure_patterns.forEach((p, i) => {
      html += '<tr><td>' + (i+1) + '</td><td>' + p.pattern + '</td><td>' + p.count + '</td></tr>';
    });
    html += '</table>';
  }

  html += '<p style="margin-top:2rem"><a href="' + API + '/simulations/' + id + '/report?format=html" target="_blank" class="btn">View Full HTML Report</a></p>';
  html += '<p style="margin-top:0.5rem"><a href="#" onclick="showTab(\'dashboard\')" style="color:var(--fg2)">← Back to dashboard</a></p>';

  el.innerHTML = html;
}

async function loadPersonas() {
  const personas = await fetchJSON(API + '/personas');
  const el = document.getElementById('persona-list');
  const checks = document.getElementById('persona-checks');

  if (!personas || personas.length === 0) {
    el.innerHTML = '<div class="empty">No personas found.</div>';
    return;
  }

  el.innerHTML = '<table><tr><th>Name</th><th>Description</th><th>Goal</th><th>Patience</th></tr>' +
    personas.map(p =>
      '<tr><td><strong>' + p.name + '</strong></td><td>' + p.description.substring(0, 120) + '...</td><td>' + p.goal.substring(0, 80) + '...</td><td>' + p.behavior.patience + '</td></tr>'
    ).join('') + '</table>';

  checks.innerHTML = personas.map((p, i) =>
    '<label style="color:var(--fg);cursor:pointer"><input type="checkbox" value="' + p.name + '"' + (i < 3 ? ' checked' : '') + '> ' + p.name + '</label>'
  ).join('');
}

async function startSimulation() {
  const target = document.getElementById('f-target').value;
  if (!target) { alert('Target URL is required'); return; }

  const checked = [...document.querySelectorAll('#persona-checks input:checked')].map(c => c.value);
  if (checked.length === 0) { alert('Select at least one persona'); return; }

  const body = {
    target_url: target,
    personas: checked,
    user_count: parseInt(document.getElementById('f-users').value) || 10,
    duration: document.getElementById('f-duration').value || '10m',
    max_turns: parseInt(document.getElementById('f-turns').value) || 20,
    rate_limit: parseInt(document.getElementById('f-rate').value) || 0,
    format: document.getElementById('f-format').value,
    proxy_url: document.getElementById('f-proxy').value,
  };

  const msg = document.getElementById('status-msg');
  msg.innerHTML = '<span class="spinner"></span> Starting simulation...';

  try {
    const r = await fetch(API + '/simulations', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(body),
    });
    const data = await r.json();
    msg.innerHTML = '✓ Simulation started. <a href="#" onclick="showTab(\'dashboard\');loadDashboard();">View dashboard</a>';
  } catch (e) {
    msg.innerHTML = '✗ Error: ' + e.message;
  }
}

// Init
loadDashboard();
loadPersonas();
</script>
</body>
</html>` 
