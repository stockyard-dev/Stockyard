package server

// adminHTML is the embedded single-page admin UI for Iron.
const adminHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Iron — Spec-to-Code Compiler</title>
<style>
  :root {
    --bg: #1a1510;
    --bg2: #231e16;
    --bg3: #2e261c;
    --rust: #8b4513;
    --rust-light: #a0522d;
    --cream: #d4a574;
    --cream-dim: #b08a60;
    --text: #e8dcc8;
    --text-dim: #9a8b78;
    --green: #5a8a5a;
    --red: #a04040;
    --yellow: #b8964a;
    --mono: 'SF Mono', SFMono-Regular, Consolas, 'Liberation Mono', Menlo, monospace;
  }
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background: var(--bg);
    color: var(--text);
    min-height: 100vh;
  }
  header {
    background: var(--bg2);
    border-bottom: 1px solid var(--bg3);
    padding: 1rem 2rem;
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  header h1 {
    font-size: 1.25rem;
    color: var(--cream);
    font-weight: 600;
    letter-spacing: 0.5px;
  }
  header h1 span { color: var(--rust-light); }
  header .status {
    font-size: 0.8rem;
    color: var(--text-dim);
  }
  header .status .dot {
    display: inline-block;
    width: 8px; height: 8px;
    border-radius: 50%;
    background: var(--green);
    margin-right: 6px;
    vertical-align: middle;
  }
  .container {
    max-width: 1200px;
    margin: 0 auto;
    padding: 1.5rem 2rem;
  }
  .tabs {
    display: flex;
    gap: 0;
    margin-bottom: 1.5rem;
    border-bottom: 1px solid var(--bg3);
  }
  .tab {
    padding: 0.6rem 1.2rem;
    background: none;
    border: none;
    color: var(--text-dim);
    cursor: pointer;
    font-size: 0.9rem;
    border-bottom: 2px solid transparent;
    transition: all 0.15s;
  }
  .tab:hover { color: var(--cream); }
  .tab.active {
    color: var(--cream);
    border-bottom-color: var(--rust);
  }
  .panel { display: none; }
  .panel.active { display: block; }
  .card {
    background: var(--bg2);
    border: 1px solid var(--bg3);
    border-radius: 6px;
    padding: 1rem 1.25rem;
    margin-bottom: 0.75rem;
  }
  .card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 0.5rem;
  }
  .card-title {
    font-size: 1rem;
    font-weight: 600;
    color: var(--cream);
  }
  .card-meta {
    font-size: 0.8rem;
    color: var(--text-dim);
  }
  .badge {
    display: inline-block;
    padding: 0.15rem 0.5rem;
    border-radius: 3px;
    font-size: 0.75rem;
    font-weight: 500;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
  .badge-yaml { background: var(--bg3); color: var(--yellow); }
  .badge-json { background: var(--bg3); color: var(--cream); }
  .badge-success { background: rgba(90,138,90,0.2); color: var(--green); }
  .badge-failed { background: rgba(160,64,64,0.2); color: var(--red); }
  .badge-pending { background: rgba(184,150,74,0.2); color: var(--yellow); }
  .badge-running { background: rgba(139,69,19,0.2); color: var(--rust-light); }
  .btn {
    padding: 0.4rem 0.9rem;
    border: 1px solid var(--bg3);
    border-radius: 4px;
    background: var(--bg3);
    color: var(--text);
    cursor: pointer;
    font-size: 0.8rem;
    transition: all 0.15s;
  }
  .btn:hover { background: var(--rust); border-color: var(--rust); }
  .btn-primary {
    background: var(--rust);
    border-color: var(--rust);
    color: var(--text);
  }
  .btn-primary:hover { background: var(--rust-light); border-color: var(--rust-light); }
  .btn-danger { color: var(--red); }
  .btn-danger:hover { background: var(--red); color: var(--text); border-color: var(--red); }
  .btn-group { display: flex; gap: 0.4rem; }
  .form-group { margin-bottom: 1rem; }
  .form-group label {
    display: block;
    font-size: 0.85rem;
    color: var(--cream-dim);
    margin-bottom: 0.3rem;
    font-weight: 500;
  }
  .form-group input,
  .form-group select,
  .form-group textarea {
    width: 100%;
    padding: 0.5rem 0.75rem;
    background: var(--bg);
    border: 1px solid var(--bg3);
    border-radius: 4px;
    color: var(--text);
    font-size: 0.9rem;
    font-family: inherit;
  }
  .form-group textarea {
    font-family: var(--mono);
    font-size: 0.85rem;
    min-height: 300px;
    resize: vertical;
    line-height: 1.5;
  }
  .form-group input:focus,
  .form-group select:focus,
  .form-group textarea:focus {
    outline: none;
    border-color: var(--rust);
  }
  .empty {
    text-align: center;
    padding: 3rem;
    color: var(--text-dim);
  }
  .empty p { margin-bottom: 1rem; }
  .code-view {
    background: var(--bg);
    border: 1px solid var(--bg3);
    border-radius: 4px;
    padding: 1rem;
    font-family: var(--mono);
    font-size: 0.8rem;
    line-height: 1.6;
    overflow-x: auto;
    white-space: pre-wrap;
    word-break: break-word;
    max-height: 500px;
    overflow-y: auto;
    color: var(--cream-dim);
  }
  .file-list { margin-top: 0.75rem; }
  .file-item {
    padding: 0.5rem 0;
    border-bottom: 1px solid var(--bg3);
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .file-item:hover { color: var(--cream); }
  .file-item:last-child { border-bottom: none; }
  .file-icon { color: var(--rust-light); font-size: 0.8rem; }
  .back-link {
    color: var(--cream-dim);
    cursor: pointer;
    font-size: 0.85rem;
    margin-bottom: 1rem;
    display: inline-block;
  }
  .back-link:hover { color: var(--cream); }
  .section-title {
    font-size: 0.9rem;
    color: var(--cream-dim);
    margin: 1rem 0 0.5rem 0;
    font-weight: 500;
  }
  .row { display: flex; gap: 0.5rem; }
  .row > * { flex: 1; }
  @media (max-width: 768px) {
    .container { padding: 1rem; }
    header { padding: 0.75rem 1rem; }
    .row { flex-direction: column; }
  }
</style>
</head>
<body>
<header>
  <h1><span>&#9881;</span> Iron <span style="font-weight:300;color:var(--text-dim)">Spec-to-Code Compiler</span></h1>
  <div class="status"><span class="dot"></span>Server running</div>
</header>

<div class="container">
  <div class="tabs">
    <button class="tab active" onclick="showTab('specs')">Specs</button>
    <button class="tab" onclick="showTab('builds')">Builds</button>
    <button class="tab" onclick="showTab('upload')">New Spec</button>
  </div>

  <!-- Specs Panel -->
  <div id="panel-specs" class="panel active">
    <div id="specs-list"></div>
  </div>

  <!-- Builds Panel -->
  <div id="panel-builds" class="panel">
    <div id="builds-list"></div>
  </div>

  <!-- Upload Panel -->
  <div id="panel-upload" class="panel">
    <div class="card">
      <div class="card-header"><div class="card-title">Upload New Spec</div></div>
      <form id="upload-form" onsubmit="createSpec(event)">
        <div class="row">
          <div class="form-group">
            <label>Spec Name</label>
            <input type="text" id="spec-name" placeholder="my-app" required>
          </div>
          <div class="form-group">
            <label>Format</label>
            <select id="spec-format">
              <option value="yaml">YAML</option>
              <option value="json">JSON</option>
            </select>
          </div>
        </div>
        <div class="form-group">
          <label>Spec Content</label>
          <textarea id="spec-content" placeholder="Paste your YAML or JSON spec here..." required></textarea>
        </div>
        <button type="submit" class="btn btn-primary">Create Spec</button>
      </form>
    </div>
  </div>

  <!-- Spec Detail (hidden by default) -->
  <div id="panel-spec-detail" class="panel">
    <div class="back-link" onclick="showTab('specs')">&larr; Back to specs</div>
    <div id="spec-detail-content"></div>
  </div>

  <!-- Build Detail (hidden by default) -->
  <div id="panel-build-detail" class="panel">
    <div class="back-link" onclick="showTab('builds')">&larr; Back to builds</div>
    <div id="build-detail-content"></div>
  </div>
</div>

<script>
function showTab(name) {
  document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
  document.querySelectorAll('.panel').forEach(p => p.classList.remove('active'));
  const panel = document.getElementById('panel-' + name);
  if (panel) panel.classList.add('active');
  document.querySelectorAll('.tab').forEach(t => {
    if (t.textContent.toLowerCase().replace(/\s+/g,'') === name.replace(/-/g,'') ||
        (name === 'upload' && t.textContent === 'New Spec'))
      t.classList.add('active');
  });
  if (name === 'specs') loadSpecs();
  if (name === 'builds') loadBuilds();
}

function fmtDate(d) {
  if (!d || d === '0001-01-01T00:00:00Z') return '—';
  const dt = new Date(d);
  return dt.toLocaleDateString() + ' ' + dt.toLocaleTimeString([], {hour:'2-digit',minute:'2-digit'});
}

function statusBadge(s) {
  return '<span class="badge badge-' + s + '">' + s + '</span>';
}

// ===== Specs =====
async function loadSpecs() {
  try {
    const res = await fetch('/api/specs');
    const specs = await res.json();
    const el = document.getElementById('specs-list');
    if (!specs || specs.length === 0) {
      el.innerHTML = '<div class="empty"><p>No specs yet.</p><button class="btn btn-primary" onclick="showTab(\'upload\')">Upload a Spec</button></div>';
      return;
    }
    el.innerHTML = specs.map(s => '<div class="card">' +
      '<div class="card-header">' +
        '<div><span class="card-title" style="cursor:pointer" onclick="viewSpec(\'' + s.id + '\')">' + esc(s.name) + '</span> ' +
        '<span class="badge badge-' + s.format + '">' + s.format + '</span></div>' +
        '<div class="card-meta">' + fmtDate(s.updated_at) + '</div>' +
      '</div>' +
      '<div style="display:flex;justify-content:space-between;align-items:center">' +
        '<div class="card-meta">ID: ' + s.id.substring(0,12) + '</div>' +
        '<div class="btn-group">' +
          '<button class="btn btn-primary" onclick="buildSpec(\'' + s.id + '\')">Build</button>' +
          '<button class="btn" onclick="viewSpec(\'' + s.id + '\')">View</button>' +
          '<button class="btn btn-danger" onclick="deleteSpec(\'' + s.id + '\',\'' + esc(s.name) + '\')">Delete</button>' +
        '</div>' +
      '</div>' +
    '</div>').join('');
  } catch (e) {
    document.getElementById('specs-list').innerHTML = '<div class="empty"><p>Error loading specs: ' + esc(e.message) + '</p></div>';
  }
}

async function viewSpec(id) {
  try {
    const res = await fetch('/api/specs/' + id);
    const s = await res.json();
    if (s.error) { alert(s.error); return; }
    document.querySelectorAll('.panel').forEach(p => p.classList.remove('active'));
    document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    document.getElementById('panel-spec-detail').classList.add('active');
    document.getElementById('spec-detail-content').innerHTML =
      '<div class="card">' +
        '<div class="card-header">' +
          '<div class="card-title">' + esc(s.name) + ' <span class="badge badge-' + s.format + '">' + s.format + '</span></div>' +
          '<div class="btn-group">' +
            '<button class="btn btn-primary" onclick="buildSpec(\'' + s.id + '\')">Build</button>' +
            '<button class="btn btn-danger" onclick="deleteSpec(\'' + s.id + '\',\'' + esc(s.name) + '\')">Delete</button>' +
          '</div>' +
        '</div>' +
        '<div class="card-meta" style="margin-bottom:0.75rem">Created: ' + fmtDate(s.created_at) + ' | Updated: ' + fmtDate(s.updated_at) + '</div>' +
        '<div class="code-view">' + esc(s.content) + '</div>' +
      '</div>';
  } catch (e) { alert('Error: ' + e.message); }
}

async function createSpec(e) {
  e.preventDefault();
  const name = document.getElementById('spec-name').value.trim();
  const content = document.getElementById('spec-content').value;
  const format = document.getElementById('spec-format').value;
  if (!name || !content) return;
  try {
    const res = await fetch('/api/specs', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({name, content, format})
    });
    const data = await res.json();
    if (data.error) { alert(data.error); return; }
    document.getElementById('spec-name').value = '';
    document.getElementById('spec-content').value = '';
    showTab('specs');
  } catch (e) { alert('Error: ' + e.message); }
}

async function deleteSpec(id, name) {
  if (!confirm('Delete spec "' + name + '"? This will also delete all associated builds.')) return;
  try {
    await fetch('/api/specs/' + id, {method: 'DELETE'});
    showTab('specs');
  } catch (e) { alert('Error: ' + e.message); }
}

async function buildSpec(id) {
  try {
    const res = await fetch('/api/specs/' + id + '/build', {method: 'POST'});
    const data = await res.json();
    if (data.error) { alert(data.error); return; }
    showTab('builds');
  } catch (e) { alert('Error: ' + e.message); }
}

// ===== Builds =====
async function loadBuilds() {
  try {
    const res = await fetch('/api/builds');
    const builds = await res.json();
    const el = document.getElementById('builds-list');
    if (!builds || builds.length === 0) {
      el.innerHTML = '<div class="empty"><p>No builds yet. Create a spec and trigger a build.</p></div>';
      return;
    }
    el.innerHTML = builds.map(b => '<div class="card">' +
      '<div class="card-header">' +
        '<div><span class="card-title" style="cursor:pointer" onclick="viewBuild(\'' + b.id + '\')">Build ' + b.id.substring(0,8) + '</span> ' +
        statusBadge(b.status) + '</div>' +
        '<div class="card-meta">' + fmtDate(b.started_at) + '</div>' +
      '</div>' +
      '<div style="display:flex;justify-content:space-between;align-items:center">' +
        '<div class="card-meta">Spec: ' + b.spec_id.substring(0,12) +
          (b.error ? ' | Error: ' + esc(b.error).substring(0,80) : '') + '</div>' +
        '<button class="btn" onclick="viewBuild(\'' + b.id + '\')">Details</button>' +
      '</div>' +
    '</div>').join('');
  } catch (e) {
    document.getElementById('builds-list').innerHTML = '<div class="empty"><p>Error loading builds: ' + esc(e.message) + '</p></div>';
  }
}

async function viewBuild(id) {
  try {
    const res = await fetch('/api/builds/' + id);
    const b = await res.json();
    if (b.error && !b.id) { alert(b.error); return; }
    document.querySelectorAll('.panel').forEach(p => p.classList.remove('active'));
    document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    document.getElementById('panel-build-detail').classList.add('active');

    let filesHTML = '';
    if (b.files && b.files.length > 0) {
      filesHTML = '<div class="section-title">Generated Files</div><div class="file-list">' +
        b.files.map((f, i) => '<div class="file-item" onclick="toggleFile(this,' + i + ')">' +
          '<span class="file-icon">&#128196;</span> ' + esc(f.path) +
          '</div><div class="code-view" id="file-code-' + i + '" style="display:none;margin-bottom:0.5rem">' + esc(f.content) + '</div>'
        ).join('') + '</div>';
    }

    document.getElementById('build-detail-content').innerHTML =
      '<div class="card">' +
        '<div class="card-header">' +
          '<div class="card-title">Build ' + b.id.substring(0,8) + ' ' + statusBadge(b.status) + '</div>' +
        '</div>' +
        '<div class="card-meta">Spec: ' + b.spec_id + '</div>' +
        '<div class="card-meta">Started: ' + fmtDate(b.started_at) + ' | Completed: ' + fmtDate(b.completed_at) + '</div>' +
        '<div class="card-meta">Output: ' + esc(b.output_dir || '—') + '</div>' +
        (b.error ? '<div class="card-meta" style="color:var(--red);margin-top:0.5rem">Error: ' + esc(b.error) + '</div>' : '') +
        filesHTML +
      '</div>';
  } catch (e) { alert('Error: ' + e.message); }
}

function toggleFile(el, idx) {
  const code = document.getElementById('file-code-' + idx);
  code.style.display = code.style.display === 'none' ? 'block' : 'none';
}

function esc(s) {
  if (!s) return '';
  const d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
}

// Initial load
loadSpecs();
</script>
</body>
</html>`
