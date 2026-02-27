// ─── Overview ──────────────────────────────────────────────────────
function OverviewView(){
  const[apps,setApps]=useState(null);const[users,setUsers]=useState([]);const[mods,setMods]=useState(0);
  useEffect(()=>{(async()=>{const a=await api('/api/apps');setApps(a.apps||[]);const u=await api('/api/auth/users');setUsers(u.users||[]);const m=await api('/api/proxy/modules');setMods(m.count||0)})()},[]);
  if(!apps)return html`<div class="loading">Loading\u2026</div>`;
  return html`<div class="page-head"><div class="page-eyebrow">Console</div><h2>System Overview</h2><p class="page-sub">All 6 apps running on a single binary.</p></div>
    <div class="stats-row"><${Stat} label="Apps" value=${apps.length} accent/><${Stat} label="Users" value=${users.length}/><${Stat} label="Modules" value=${mods}/><${Stat} label="Status" value="Operational" accent/></div>
    <div class="app-grid">${apps.map(a=>{const cfg=APPS[a.name];return html`<div key=${a.name} class="app-card"><div class="app-card-head"><span class="app-card-icon">${cfg?.icon||'\u25C6'}</span><span class="app-card-name">${a.name}</span><${Badge} text="live" variant="success"/></div><div class="app-card-desc">${a.description}</div><code class="app-card-api">${a.api}</code></div>`})}</div>`;
}

