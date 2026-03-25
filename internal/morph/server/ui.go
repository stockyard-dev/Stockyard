package server

const adminHTML = `<!DOCTYPE html>
<html><head><meta charset="UTF-8"><title>Stockyard Morph</title>
<style>
:root{--bg:#1a1510;--bg2:#2a2318;--bg3:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--cream:#d4a574;--green:#66bb6a;--red:#ef5350;--yellow:#ffa726}
*{margin:0;padding:0;box-sizing:border-box}body{font-family:Georgia,serif;background:var(--bg);color:var(--fg);min-height:100vh}
.header{background:var(--bg2);border-bottom:2px solid var(--rust);padding:1rem 2rem;display:flex;align-items:center;gap:1rem}
.header h1{font-size:1.4rem;color:var(--cream)}.badge{background:var(--rust);color:var(--fg);font-size:.7rem;padding:.2rem .5rem;border-radius:3px;font-family:monospace}
.container{max-width:1000px;margin:0 auto;padding:2rem}
h2{color:var(--cream);margin:2rem 0 1rem;border-bottom:1px solid var(--bg3);padding-bottom:.5rem}
textarea{width:100%;background:var(--bg);border:1px solid var(--bg3);color:var(--fg);padding:.6rem;border-radius:4px;font-family:Georgia;font-size:1rem;resize:vertical;min-height:60px}
.btn{background:var(--rust);color:var(--fg);border:none;padding:.6rem 1.2rem;border-radius:4px;cursor:pointer;font-family:Georgia;font-size:.95rem;margin-top:.5rem}
.btn:hover{opacity:.85}.btn-secondary{background:var(--bg3)}
pre{background:#111;padding:1rem;border-radius:4px;font-size:.8rem;color:#ccc;overflow-x:auto;margin:1rem 0;max-height:400px;overflow-y:auto}
table{width:100%;border-collapse:collapse}th{background:var(--bg3);padding:.4rem .6rem;text-align:left;font-family:monospace;font-size:.75rem}
td{padding:.4rem .6rem;border-bottom:1px solid var(--bg3);font-size:.85rem}
.configured{color:var(--green)}.unconfigured{color:var(--fg2)}
.stats-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:1rem;margin:1rem 0}
.stat-card{background:var(--bg2);border:1px solid var(--bg3);border-radius:6px;padding:1.2rem;text-align:center}
.stat-card .value{font-size:2rem;font-weight:bold;color:var(--cream)}
.stat-card .label{font-size:.8rem;color:var(--fg2);margin-top:.3rem}
.status-ok{color:var(--green)}.status-err{color:var(--red)}.status-warn{color:var(--yellow)}
.toggle-group{display:flex;gap:.5rem;margin:.5rem 0;align-items:center}
.toggle-group label{font-size:.9rem;color:var(--fg2);cursor:pointer}
.toggle{appearance:none;width:40px;height:22px;background:var(--bg3);border-radius:11px;position:relative;cursor:pointer;border:none;outline:none}
.toggle:checked{background:var(--rust)}
.toggle::after{content:'';position:absolute;top:3px;left:3px;width:16px;height:16px;background:var(--fg);border-radius:50%;transition:.2s}
.toggle:checked::after{left:21px}
.svc-breakdown{margin:1rem 0}
.svc-bar{display:flex;align-items:center;gap:.5rem;margin:.3rem 0;font-size:.85rem}
.svc-bar .bar{height:18px;background:var(--rust);border-radius:3px;min-width:4px}
.svc-bar .svc-name{width:80px;text-align:right;color:var(--fg2);font-family:monospace;font-size:.75rem}
.tabs{display:flex;gap:0;margin-bottom:1rem;border-bottom:2px solid var(--bg3)}
.tab{padding:.5rem 1rem;cursor:pointer;color:var(--fg2);border-bottom:2px solid transparent;margin-bottom:-2px;font-family:Georgia}
.tab.active{color:var(--cream);border-bottom-color:var(--rust)}
.tab-content{display:none}.tab-content.active{display:block}
</style></head><body>
<div class="header"><h1>Morph</h1><span class="badge">Universal API Translator</span></div>
<div class="container">

<div class="stats-grid" id="stats-grid">
  <div class="stat-card"><div class="value" id="stat-total">-</div><div class="label">Total Executions</div></div>
  <div class="stat-card"><div class="value" id="stat-rate">-</div><div class="label">Success Rate</div></div>
  <div class="stat-card"><div class="value" id="stat-latency">-</div><div class="label">Avg Latency</div></div>
  <div class="stat-card"><div class="value" id="stat-services">-</div><div class="label">Services</div></div>
</div>

<div class="tabs">
  <div class="tab active" data-tab="execute">Execute</div>
  <div class="tab" data-tab="history">History</div>
  <div class="tab" data-tab="services">Services</div>
</div>

<div id="tab-execute" class="tab-content active">
<textarea id="intent" placeholder='charge alice@example.com $49.99 for the Pro plan'></textarea>
<div class="toggle-group">
  <input type="checkbox" class="toggle" id="dry-run-toggle">
  <label for="dry-run-toggle">Dry-run mode (preview without executing)</label>
</div>
<button class="btn" onclick="doIt()">Execute</button>
<button class="btn btn-secondary" onclick="doDryRun()">Preview</button>
<pre id="result" style="display:none"></pre>
</div>

<div id="tab-history" class="tab-content">
<table><thead><tr><th>Time</th><th>Service</th><th>Action</th><th>Method</th><th>Status</th><th>Latency</th><th>Intent</th></tr></thead>
<tbody id="history"></tbody></table>
</div>

<div id="tab-services" class="tab-content">
<h2 style="margin-top:0">Per-Service Breakdown</h2>
<div class="svc-breakdown" id="svc-breakdown"></div>
<h2>Available Services</h2>
<table><thead><tr><th>Service</th><th>Actions</th><th>Status</th></tr></thead>
<tbody id="services"></tbody></table>
</div>

</div>
<script>
const API=location.origin+'/api';

// Tabs
document.querySelectorAll('.tab').forEach(t=>{
  t.addEventListener('click',()=>{
    document.querySelectorAll('.tab').forEach(x=>x.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(x=>x.classList.remove('active'));
    t.classList.add('active');
    document.getElementById('tab-'+t.dataset.tab).classList.add('active');
  });
});

async function doIt(){
  const isDry=document.getElementById('dry-run-toggle').checked;
  if(isDry){doDryRun();return;}
  const intent=document.getElementById('intent').value;
  if(!intent)return;
  const pre=document.getElementById('result');
  pre.style.display='block';pre.textContent='Executing...';
  try{
    const r=await fetch(API+'/do',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({intent})});
    const d=await r.json();pre.textContent=JSON.stringify(d,null,2);
    loadStats();loadHistory();
  }catch(e){pre.textContent='Error: '+e.message;}
}

async function doDryRun(){
  const intent=document.getElementById('intent').value;
  if(!intent)return;
  const pre=document.getElementById('result');
  pre.style.display='block';pre.textContent='Resolving...';
  try{
    const r=await fetch(API+'/dry-run',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({intent})});
    const d=await r.json();pre.textContent='[DRY RUN]\n'+JSON.stringify(d,null,2);
  }catch(e){pre.textContent='Error: '+e.message;}
}

async function loadStats(){
  try{
    const s=await(await fetch(API+'/stats')).json();
    document.getElementById('stat-total').textContent=s.total_executions||0;
    document.getElementById('stat-rate').textContent=s.total_executions?(s.success_rate*100).toFixed(1)+'%':'--';
    document.getElementById('stat-latency').textContent=s.total_executions?s.avg_latency_ms.toFixed(0)+'ms':'--';
    document.getElementById('stat-services').textContent=(s.per_service||[]).length;
    // Service breakdown bars
    const bd=document.getElementById('svc-breakdown');
    const ps=s.per_service||[];
    const maxC=Math.max(...ps.map(p=>p.count),1);
    bd.innerHTML=ps.map(p=>'<div class="svc-bar"><span class="svc-name">'+p.service+'</span><div class="bar" style="width:'+((p.count/maxC)*300)+'px"></div><span>'+p.count+' calls, '+(p.success_rate*100).toFixed(0)+'% ok, '+p.avg_latency_ms.toFixed(0)+'ms avg</span></div>').join('');
  }catch(e){}
}

async function loadHistory(){
  try{
    const h=await(await fetch(API+'/history?limit=50')).json();
    document.getElementById('history').innerHTML=(h||[]).map(x=>{
      const sc=x.status_code>=200&&x.status_code<300?'status-ok':(x.status_code>=400?'status-err':'status-warn');
      const t=x.created_at?new Date(x.created_at).toLocaleTimeString():'';
      return '<tr><td>'+t+'</td><td>'+x.service+'</td><td>'+x.action+'</td><td><code>'+x.method+'</code></td><td class="'+sc+'">'+x.status_code+'</td><td>'+x.latency_ms.toFixed(0)+'ms</td><td style="max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">'+x.intent+'</td></tr>';
    }).join('');
  }catch(e){}
}

async function loadServices(){
  try{
    const s=await(await fetch(API+'/services')).json();
    document.getElementById('services').innerHTML=(s||[]).map(x=>
      '<tr><td><strong>'+x.name+'</strong></td><td>'+x.actions.join(', ')+'</td><td class="'+(x.configured?'configured':'unconfigured')+'">'+(x.configured?'Configured':'Not configured')+'</td></tr>'
    ).join('');
  }catch(e){}
}

loadStats();loadHistory();loadServices();
</script></body></html>`
