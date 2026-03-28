// ─── Overview ──────────────────────────────────────────────────────
function OverviewView(){
  const[apps,setApps]=useState(null);const[users,setUsers]=useState([]);const[mods,setMods]=useState(0);
  const[platform,setPlatform]=useState(null);const[bus,setBus]=useState(null);const[relic,setRelic]=useState(null);const[crucible,setCrucible]=useState(null);const[costs,setCosts]=useState(null);
  const load=async()=>{
    const a=await api('/api/apps');setApps(a.apps||[]);
    const u=await api('/api/auth/users');setUsers(u.users||[]);
    const m=await api('/api/proxy/modules');setMods(m.count||0);
    const p=await api('/api/platform/health');if(!p._error)setPlatform(p);
    const b=await api('/api/platform/bus');if(!b._error)setBus(b);
    const r=await api('/relic/api/stats');if(!r._error)setRelic(r);
    const c=await api('/crucible/api/stats');if(!c._error)setCrucible(c);
    const co=await api('/api/observe/costs');if(!co._error)setCosts(co);
  };
  useEffect(()=>{load();const i=setInterval(load,15000);return()=>clearInterval(i)},[]);
  if(!apps)return html`<div class="loading">Loading\u2026</div>`;
  const totalCost=costs?.providers?.reduce((s,p)=>s+(p.cost_usd||0),0)||0;
  const totalReqs=costs?.providers?.reduce((s,p)=>s+(p.requests||0),0)||0;
  const providerCount=costs?.providers?.length||0;
  const isFirstRun=totalReqs===0;
  return html`<div class="page-head"><div class="page-eyebrow">Console</div><h2>System Overview</h2><p class="page-sub">${platform?platform.products+' products':apps.length+' apps'} \u2022 ${platform?.tier||'Community'} tier \u2022 ${platform?.status||'ok'}</p></div>
    <div class="stats-row">
      <${Stat} label="Products" value=${platform?.active||apps.length} sub=${platform?(platform.healthy+' healthy'):''} accent/>
      <${Stat} label="Requests" value=${fmt.num(totalReqs)} sub=${fmt.usd(totalCost)+' spent'}/>
      <${Stat} label="Modules" value=${mods}/>
      <${Stat} label="Providers" value=${providerCount} sub=${providerCount>0?'configured':'none yet'}/>
    </div>
    ${isFirstRun?html`<div class="first-run-checklist">
      <div class="frc-title">\u{1F680} Getting started</div>
      <div class="frc-item ${providerCount>0?'frc-done':''}"><span class="frc-check">${providerCount>0?'\u2713':'\u25CB'}</span><span>Set a provider key</span><code class="frc-code">export OPENAI_API_KEY=sk-...</code></div>
      <div class="frc-item"><span class="frc-check">\u25CB</span><span>Send your first request</span><code class="frc-code">curl http://localhost:7749/v1/chat/completions -H "Content-Type: application/json" -H "Authorization: Bearer $OPENAI_API_KEY" -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}'</code></div>
      <div class="frc-item"><span class="frc-check">\u25CB</span><span>Point your app at the proxy</span><code class="frc-code">export OPENAI_BASE_URL=http://localhost:7749/v1</code></div>
      <div class="frc-item"><span class="frc-check">\u25CB</span><span>Check traces in Observe tab</span></div>
      <div class="frc-hint">Once you send a request, this checklist disappears and your live dashboard takes over.</div>
    </div>`:null}
    ${relic||crucible?html`<div class="stats-row" style="margin-top:12px">
      ${relic?html`<${Stat} label="Relic Certs" value=${relic.total_certificates||0} sub=${'conf '+(relic.avg_confidence||0).toFixed(2)}/>`:null}
      ${crucible?html`<${Stat} label="Crucible Scores" value=${crucible.total_scores||0} sub=${'avg '+(crucible.avg_compound_score||0).toFixed(3)}/>`:null}
      <${Stat} label="Users" value=${users.length}/>
      <${Stat} label="Workflows" value="10" sub="chains active"/>
    </div>`:null}
    <div class="app-grid">${apps.map(a=>{const cfg=APPS[a.name];return html`<div key=${a.name} class="app-card"><div class="app-card-head"><span class="app-card-icon">${cfg?.icon||'\u25C6'}</span><span class="app-card-name">${a.name}</span><${Badge} text="live" variant="success"/></div><div class="app-card-desc">${a.description}</div><code class="app-card-api">${a.api}</code></div>`})}</div>`;
}

