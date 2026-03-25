package server

const adminHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Stockyard Trailhead</title>
<style>
:root {
  --bg: #1a1510; --bg2: #2a2318; --bg3: #3a3228;
  --fg: #f5f0e8; --fg2: #8a7e6e; --fg3: #5a5248;
  --rust: #8b4513; --cream: #d4a574; --gold: #f5c542;
  --green: #66bb6a; --blue: #42a5f5;
}
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: Georgia, serif; background: var(--bg); color: var(--fg); min-height: 100vh; }
a { color: var(--cream); text-decoration: none; }

.header { background: var(--bg2); border-bottom: 2px solid var(--rust); padding: 1rem 2rem; display: flex; align-items: center; gap: 1rem; }
.header h1 { font-size: 1.4rem; color: var(--cream); }
.header .badge { background: var(--rust); color: var(--fg); font-size: 0.7rem; padding: 0.2rem 0.5rem; border-radius: 3px; font-family: monospace; }

.container { max-width: 900px; margin: 0 auto; padding: 2rem; }

.cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 1rem; margin: 1.5rem 0; }
.card { background: var(--bg2); border-radius: 8px; padding: 1rem; border: 1px solid var(--bg3); text-align: center; }
.card-value { font-size: 1.8rem; font-weight: bold; color: var(--cream); }
.card-label { font-size: 0.75rem; color: var(--fg2); margin-top: 0.3rem; }

h2 { color: var(--cream); margin: 2rem 0 1rem; font-size: 1.2rem; border-bottom: 1px solid var(--bg3); padding-bottom: 0.5rem; }

.ask-box { background: var(--bg2); border-radius: 8px; padding: 1.5rem; margin: 1.5rem 0; border: 1px solid var(--bg3); }
.ask-input { width: 100%; background: var(--bg); border: 1px solid var(--bg3); color: var(--fg); padding: 0.8rem 1rem; border-radius: 6px; font-family: Georgia, serif; font-size: 1rem; outline: none; }
.ask-input:focus { border-color: var(--cream); }
.ask-input::placeholder { color: var(--fg3); }
.ask-btn { background: var(--rust); color: var(--fg); border: none; padding: 0.6rem 1.5rem; border-radius: 4px; cursor: pointer; font-family: Georgia, serif; font-size: 0.95rem; margin-top: 0.8rem; }
.ask-btn:hover { opacity: 0.85; }
.ask-btn:disabled { opacity: 0.5; cursor: not-allowed; }

.answer-box { background: var(--bg2); border-radius: 8px; padding: 1.5rem; margin: 1rem 0; border-left: 3px solid var(--cream); }
.answer-text { line-height: 1.7; white-space: pre-wrap; }
.answer-meta { color: var(--fg2); font-size: 0.8rem; margin-top: 1rem; padding-top: 0.8rem; border-top: 1px solid var(--bg3); }

.sources { margin-top: 0.8rem; }
.source-item { background: var(--bg); padding: 0.5rem 0.8rem; border-radius: 4px; margin: 0.3rem 0; font-size: 0.85rem; display: flex; justify-content: space-between; align-items: center; }
.source-score { color: var(--green); font-family: monospace; font-size: 0.8rem; }

table { width: 100%; border-collapse: collapse; margin: 1rem 0; }
th { background: var(--bg3); color: var(--fg); padding: 0.5rem 0.8rem; text-align: left; font-family: monospace; font-size: 0.8rem; }
td { padding: 0.5rem 0.8rem; border-bottom: 1px solid var(--bg3); font-size: 0.85rem; }
tr:hover td { background: var(--bg2); }

.entity-badge { display: inline-block; padding: 0.15rem 0.5rem; border-radius: 3px; font-size: 0.7rem; font-family: monospace; margin-right: 0.3rem; }
.entity-person { background: #1565C0; color: white; }
.entity-file { background: #2E7D32; color: white; }
.entity-concept { background: #6A1B9A; color: white; }
.entity-decision { background: #E65100; color: white; }

.index-form { background: var(--bg2); border-radius: 8px; padding: 1.5rem; margin: 1rem 0; border: 1px solid var(--bg3); }
.form-row { display: flex; gap: 1rem; margin-bottom: 0.8rem; }
.form-row input { flex: 1; background: var(--bg); border: 1px solid var(--bg3); color: var(--fg); padding: 0.5rem 0.8rem; border-radius: 4px; font-family: monospace; font-size: 0.9rem; }

.tab-bar { display: flex; gap: 0; margin-top: 1.5rem; border-bottom: 2px solid var(--bg3); }
.tab { padding: 0.5rem 1.2rem; cursor: pointer; color: var(--fg2); font-size: 0.9rem; border-bottom: 2px solid transparent; margin-bottom: -2px; }
.tab:hover { color: var(--fg); }
.tab.active { color: var(--cream); border-bottom-color: var(--cream); }
.tab-content { display: none; }
.tab-content.active { display: block; }

.spinner { display: inline-block; width: 1rem; height: 1rem; border: 2px solid var(--bg3); border-top-color: var(--cream); border-radius: 50%; animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
.empty { color: var(--fg3); text-align: center; padding: 2rem; font-style: italic; }
</style>
</head>
<body>

<div class="header">
  <h1>🧠 Trailhead</h1>
  <span class="badge">Codebase Intelligence</span>
</div>

<div class="container">
  <div class="cards" id="stats-cards">
    <div class="card"><div class="card-value" id="s-sources">—</div><div class="card-label">Sources</div></div>
    <div class="card"><div class="card-value" id="s-docs">—</div><div class="card-label">Documents</div></div>
    <div class="card"><div class="card-value" id="s-chunks">—</div><div class="card-label">Chunks</div></div>
    <div class="card"><div class="card-value" id="s-entities">—</div><div class="card-label">Entities</div></div>
    <div class="card"><div class="card-value" id="s-rels">—</div><div class="card-label">Relationships</div></div>
    <div class="card"><div class="card-value" id="s-queries">—</div><div class="card-label">Queries</div></div>
  </div>

  <!-- Ask Box -->
  <div class="ask-box">
    <input type="text" class="ask-input" id="question-input"
      placeholder="Ask anything about your codebase..."
      onkeydown="if(event.key==='Enter')askQuestion()" />
    <button class="ask-btn" id="ask-btn" onclick="askQuestion()">🧠 Ask Trailhead</button>
  </div>
  <div id="answer-area"></div>

  <div class="tab-bar">
    <div class="tab active" onclick="showTab('sources')">Sources</div>
    <div class="tab" onclick="showTab('entities')">Knowledge Graph</div>
    <div class="tab" onclick="showTab('history')">Query History</div>
    <div class="tab" onclick="showTab('index')">Index New</div>
  </div>

  <div id="tab-sources" class="tab-content active">
    <div id="sources-list" class="empty">Loading...</div>
  </div>

  <div id="tab-entities" class="tab-content">
    <div style="margin:1rem 0">
      <input type="text" class="ask-input" id="entity-search" placeholder="Search entities..." style="max-width:400px"
        oninput="searchEntities(this.value)" />
    </div>
    <div id="entities-list" class="empty">Loading...</div>
  </div>

  <div id="tab-history" class="tab-content">
    <div id="history-list" class="empty">Loading...</div>
  </div>

  <div id="tab-index" class="tab-content">
    <h2>Index a GitHub Repository</h2>
    <div class="index-form">
      <div class="form-row">
        <input type="text" id="idx-repo" placeholder="owner/repo (e.g. stockyard-dev/Stockyard)" />
      </div>
      <div class="form-row">
        <input type="text" id="idx-token" placeholder="GitHub token (optional, uses server default)" />
      </div>
      <button class="ask-btn" onclick="startIndex()">Index Repository</button>
      <div id="index-status" style="color:var(--fg2);font-size:0.85rem;margin-top:0.5rem;"></div>
    </div>
  </div>
</div>

<script>
const API = window.location.origin + '/api';

function showTab(name) {
  document.querySelectorAll('.tab-content').forEach(t => t.classList.remove('active'));
  document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
  document.getElementById('tab-' + name).classList.add('active');
  event.target.classList.add('active');
}

async function loadStats() {
  try {
    const data = await (await fetch(API + '/stats')).json();
    document.getElementById('s-sources').textContent = data.sources || 0;
    document.getElementById('s-docs').textContent = data.documents || 0;
    document.getElementById('s-chunks').textContent = data.chunks || 0;
    document.getElementById('s-entities').textContent = data.entities || 0;
    document.getElementById('s-rels').textContent = data.relationships || 0;
    document.getElementById('s-queries').textContent = data.queries || 0;
  } catch(e) {}
}

async function askQuestion() {
  const input = document.getElementById('question-input');
  const q = input.value.trim();
  if (!q) return;

  const btn = document.getElementById('ask-btn');
  const area = document.getElementById('answer-area');
  btn.disabled = true;
  btn.textContent = '🧠 Thinking...';
  area.innerHTML = '<div class="answer-box"><span class="spinner"></span> Searching knowledge base...</div>';

  try {
    const resp = await fetch(API + '/ask', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({question: q, top_k: 8})
    });
    const data = await resp.json();

    if (data.error) {
      area.innerHTML = '<div class="answer-box" style="border-left-color:var(--rust)">' + data.error + '</div>';
      return;
    }

    let html = '<div class="answer-box">';
    html += '<div class="answer-text">' + escapeHtml(data.response || data.answer || 'No answer generated.') + '</div>';

    // Sources
    if (data.sources && data.sources.length > 0) {
      html += '<div class="sources"><strong style="font-size:0.85rem;color:var(--fg2)">Sources:</strong>';
      data.sources.forEach(s => {
        const score = Math.round((s.relevance_score || s.score || 0) * 100);
        html += '<div class="source-item"><span>' + escapeHtml(s.title || '') + ' <span style="color:var(--fg3)">(' + (s.type||'') + ')</span></span><span class="source-score">' + score + '%</span></div>';
      });
      html += '</div>';
    }

    html += '<div class="answer-meta">' + (data.chunks_used||0) + ' chunks · $' + (data.cost||data.cost_cents||0).toFixed(4) + ' · ' + (data.latency_ms || (data.latency ? Math.round(data.latency/1e6) : 0)) + 'ms</div>';
    html += '</div>';
    area.innerHTML = html;

    loadStats(); // refresh query count
  } catch(e) {
    area.innerHTML = '<div class="answer-box" style="border-left-color:var(--rust)">Error: ' + e.message + '</div>';
  } finally {
    btn.disabled = false;
    btn.textContent = '🧠 Ask Trailhead';
  }
}

async function loadSources() {
  try {
    const data = await (await fetch(API + '/sources')).json();
    const el = document.getElementById('sources-list');
    if (!data || data.length === 0) {
      el.innerHTML = '<div class="empty">No sources indexed yet. Use the "Index New" tab to add a repository.</div>';
      return;
    }
    let html = '<table><tr><th>Type</th><th>Name</th><th>Documents</th><th>Last Synced</th></tr>';
    data.forEach(s => {
      const synced = s.last_synced_at ? new Date(s.last_synced_at).toLocaleDateString() : 'never';
      html += '<tr><td>' + s.type + '</td><td><strong>' + escapeHtml(s.name) + '</strong></td><td>' + (s.document_count||0) + '</td><td>' + synced + '</td></tr>';
    });
    html += '</table>';
    el.innerHTML = html;
  } catch(e) {
    document.getElementById('sources-list').innerHTML = '<div class="empty">Error loading sources</div>';
  }
}

async function searchEntities(q) {
  try {
    const url = q ? API + '/entities?q=' + encodeURIComponent(q) + '&limit=30' : API + '/entities?limit=30';
    const data = await (await fetch(url)).json();
    const el = document.getElementById('entities-list');
    if (!data || data.length === 0) {
      el.innerHTML = '<div class="empty">No entities found.</div>';
      return;
    }
    let html = '<table><tr><th>Name</th><th>Type</th><th>Mentions</th></tr>';
    data.forEach(e => {
      const cls = 'entity-' + (e.type || 'concept');
      html += '<tr><td><strong>' + escapeHtml(e.name) + '</strong></td><td><span class="entity-badge ' + cls + '">' + e.type + '</span></td><td>' + (e.mention_count||0) + '</td></tr>';
    });
    html += '</table>';
    el.innerHTML = html;
  } catch(e) {}
}

async function loadHistory() {
  try {
    const data = await (await fetch(API + '/queries')).json();
    const el = document.getElementById('history-list');
    if (!data || data.length === 0) {
      el.innerHTML = '<div class="empty">No queries yet. Ask a question above!</div>';
      return;
    }
    let html = '<table><tr><th>Question</th><th>Chunks</th><th>Cost</th><th>Time</th></tr>';
    data.forEach(q => {
      html += '<tr><td style="cursor:pointer;max-width:400px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" onclick="document.getElementById(\'question-input\').value=\'' + escapeHtml(q.question).replace(/'/g, "\\'") + '\';askQuestion()">' + escapeHtml(q.question) + '</td>';
      html += '<td>' + (q.chunks_used||0) + '</td><td>$' + (q.cost_cents||0).toFixed(4) + '</td><td>' + (q.latency_ms||0) + 'ms</td></tr>';
    });
    html += '</table>';
    el.innerHTML = html;
  } catch(e) {}
}

async function startIndex() {
  const repo = document.getElementById('idx-repo').value.trim();
  if (!repo) { alert('Enter a repo in owner/repo format'); return; }

  const token = document.getElementById('idx-token').value.trim();
  const status = document.getElementById('index-status');
  status.innerHTML = '<span class="spinner"></span> Starting indexing...';

  try {
    const resp = await fetch(API + '/index', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({type: 'github', repo: repo, token: token || undefined})
    });
    const data = await resp.json();
    status.textContent = data.message || 'Indexing started.';
    setTimeout(loadStats, 5000);
    setTimeout(loadSources, 10000);
  } catch(e) {
    status.textContent = 'Error: ' + e.message;
  }
}

function escapeHtml(s) {
  if (!s) return '';
  return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

// Init
loadStats();
loadSources();
searchEntities('');
loadHistory();
</script>
</body>
</html>`
