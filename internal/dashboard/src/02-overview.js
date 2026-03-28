// ─── Provider metadata ─────────────────────────────────────────────
const PROV_META={
  openai:{type:'native',env:'OPENAI_API_KEY'},anthropic:{type:'native',env:'ANTHROPIC_API_KEY'},
  gemini:{type:'native',env:'GEMINI_API_KEY'},groq:{type:'native',env:'GROQ_API_KEY'},
  mistral:{type:'compat',env:'MISTRAL_API_KEY'},together:{type:'compat',env:'TOGETHER_API_KEY'},
  deepseek:{type:'compat',env:'DEEPSEEK_API_KEY'},fireworks:{type:'compat',env:'FIREWORKS_API_KEY'},
  perplexity:{type:'compat',env:'PERPLEXITY_API_KEY'},openrouter:{type:'compat',env:'OPENROUTER_API_KEY'},
  azure:{type:'compat',env:'AZURE_OPENAI_API_KEY'},xai:{type:'compat',env:'XAI_API_KEY'},
  cohere:{type:'compat',env:'COHERE_API_KEY'},replicate:{type:'compat',env:'REPLICATE_API_TOKEN'},
  deepinfra:{type:'compat',env:'DEEPINFRA_API_KEY'},nvidia:{type:'compat',env:'NVIDIA_API_KEY'},
  cerebras:{type:'compat',env:'CEREBRAS_API_KEY'},sambanova:{type:'compat',env:'SAMBANOVA_API_KEY'},
  huggingface:{type:'compat',env:'HF_TOKEN'},hyperbolic:{type:'compat',env:'HYPERBOLIC_API_KEY'},
  novita:{type:'compat',env:'NOVITA_API_KEY'},featherless:{type:'compat',env:'FEATHERLESS_API_KEY'},
  lambda:{type:'compat',env:'LAMBDA_API_KEY'},nebius:{type:'compat',env:'NEBIUS_API_KEY'},
  lepton:{type:'compat',env:'LEPTON_API_KEY'},nscale:{type:'compat',env:'NSCALE_API_KEY'},
  ai21:{type:'compat',env:'AI21_API_KEY'},friendli:{type:'compat',env:'FRIENDLI_TOKEN'},
  moonshot:{type:'compat',env:'MOONSHOT_API_KEY'},dashscope:{type:'compat',env:'DASHSCOPE_API_KEY'},
  github:{type:'compat',env:'GITHUB_MODELS_TOKEN'},baseten:{type:'compat',env:'BASETEN_API_KEY'},
  yi:{type:'compat',env:'YI_API_KEY'},
  ollama:{type:'local',env:'localhost:11434'},lmstudio:{type:'local',env:'localhost:1234'},
  vllm:{type:'local',env:'localhost:8000'},sglang:{type:'local',env:'localhost:30000'},
  tgi:{type:'local',env:'localhost:8080'}
};
function provBadge(type){const m={native:{t:'Native',v:'success'},compat:{t:'Compat',v:''},local:{t:'Local',v:'warn'}};const b=m[type]||m.compat;return html`<${Badge} text=${b.t} variant=${b.v}/>`}
function provStatus(s){const m={ok:{icon:'\u2713',cls:'prov-ok'},healthy:{icon:'\u2713',cls:'prov-ok'},active:{icon:'\u2713',cls:'prov-ok'},unconfigured:{icon:'\u25CB',cls:'prov-unconfig'},unreachable:{icon:'\u2717',cls:'prov-err'},degraded:{icon:'\u26A0',cls:'prov-warn'}};const r=m[s]||{icon:'?',cls:'prov-unconfig'};return html`<span class="prov-status ${r.cls}">${r.icon}</span>`}

// ─── Provider panel component ──────────────────────────────────────
function ProviderPanel({provHealth,providerCount}){
  const[expanded,setExpanded]=useState(false);
  const[testing,setTesting]=useState(null);
  const testOne=async(name)=>{setTesting(name);try{await api('/api/proxy/providers/health')}catch(e){}setTesting(null)};
  const configured=provHealth?.providers?.filter(p=>p.status!=='unconfigured')||[];
  const unconfigured=provHealth?.providers?.filter(p=>p.status==='unconfigured')||[];
  const healthyCount=configured.filter(p=>p.status==='ok'||p.status==='healthy'||p.status==='active').length;
  if(!provHealth||!provHealth.providers)return null;
  return html`<div class="prov-panel">
    <div class="prov-panel-head" onClick=${()=>setExpanded(!expanded)}>
      <span class="prov-panel-title">\u25C8 Providers</span>
      <span class="prov-panel-summary">${configured.length} configured${healthyCount>0?' \u2022 '+healthyCount+' healthy':''} / 40 available</span>
      <span class="prov-panel-toggle">${expanded?'\u25B4':'\u25BE'}</span>
    </div>
    ${expanded?html`<div class="prov-panel-body">
      ${configured.length>0?html`<div class="prov-panel-section">
        <div class="prov-panel-section-title">Configured</div>
        ${configured.map(p=>{const meta=PROV_META[p.provider]||{type:'compat',env:'?'};return html`<div key=${p.provider} class="prov-row">
          ${provStatus(p.status)}
          <span class="prov-row-name mono">${p.provider}</span>
          ${provBadge(meta.type)}
          ${p.latency_ms>0?html`<span class="prov-row-latency mono">${p.latency_ms}ms</span>`:null}
          ${p.error?html`<span class="prov-row-error">${p.error.substring(0,60)}</span>`:null}
        </div>`})}
      </div>`:html`<div class="prov-panel-empty">No providers configured. Set an API key environment variable to get started.</div>`}
      ${unconfigured.length>0?html`<details class="prov-available">
        <summary class="prov-available-title">${40-configured.length} available providers</summary>
        <div class="prov-available-grid">${Object.entries(PROV_META).filter(([k])=>!configured.find(c=>c.provider===k)).map(([name,meta])=>html`<div key=${name} class="prov-avail-item">
          <span class="mono">${name}</span>
          ${provBadge(meta.type)}
          <span class="prov-avail-env mono">${meta.env}</span>
        </div>`)}</div>
      </details>`:null}
    </div>`:null}
  </div>`;
}

// ─── Overview ──────────────────────────────────────────────────────
function OverviewView(){
  const[apps,setApps]=useState(null);const[users,setUsers]=useState([]);const[mods,setMods]=useState(0);
  const[platform,setPlatform]=useState(null);const[bus,setBus]=useState(null);const[relic,setRelic]=useState(null);const[crucible,setCrucible]=useState(null);const[costs,setCosts]=useState(null);
  const[provHealth,setProvHealth]=useState(null);
  const load=async()=>{
    const a=await api('/api/apps');setApps(a.apps||[]);
    const u=await api('/api/auth/users');setUsers(u.users||[]);
    const m=await api('/api/proxy/modules');setMods(m.count||0);
    const p=await api('/api/platform/health');if(!p._error)setPlatform(p);
    const b=await api('/api/platform/bus');if(!b._error)setBus(b);
    const r=await api('/relic/api/stats');if(!r._error)setRelic(r);
    const c=await api('/crucible/api/stats');if(!c._error)setCrucible(c);
    const co=await api('/api/observe/costs');if(!co._error)setCosts(co);
    const ph=await api('/api/proxy/providers/health');if(!ph._error)setProvHealth(ph);
  };
  useEffect(()=>{load();const i=setInterval(load,15000);return()=>clearInterval(i)},[]);
  if(!apps)return html`<div class="loading">Loading\u2026</div>`;
  const totalCost=costs?.providers?.reduce((s,p)=>s+(p.cost_usd||0),0)||0;
  const totalReqs=costs?.providers?.reduce((s,p)=>s+(p.requests||0),0)||0;
  const providerCount=costs?.providers?.length||0;
  const isFirstRun=totalReqs===0;
  const[testResult,setTestResult]=useState(null);const[testing,setTesting]=useState(false);
  const testConnection=async()=>{setTesting(true);setTestResult(null);try{const r=await api('/api/proxy/providers/health');const ok=!r._error&&r.providers?.some(p=>p.status==='active'||p.status==='ok'||p.reachable);setTestResult(ok?'ok':'fail')}catch(e){setTestResult('fail')}setTesting(false)};
  return html`<div class="page-head"><div class="page-eyebrow">Console</div><h2>System Overview</h2><p class="page-sub">${platform?platform.products+' products':apps.length+' apps'} \u2022 ${platform?.tier||'Community'} tier \u2022 ${platform?.status||'ok'}</p></div>
    <div class="stats-row">
      <${Stat} label="Products" value=${platform?.active||apps.length} sub=${platform?(platform.healthy+' healthy'):''} accent/>
      <${Stat} label="Requests" value=${fmt.num(totalReqs)} sub=${fmt.usd(totalCost)+' spent'}/>
      <${Stat} label="Modules" value=${mods}/>
      <${Stat} label="Providers" value=${providerCount} sub=${providerCount>0?'configured':'none yet'}/>
    </div>
    <${ProviderPanel} provHealth=${provHealth} providerCount=${providerCount}/>
    ${isFirstRun?html`<div class="first-run-checklist">
      <div class="frc-title">\u{1F680} Getting started</div>
      <div class="frc-item ${providerCount>0?'frc-done':''}"><span class="frc-check">${providerCount>0?'\u2713':'\u25CB'}</span><span>Set a provider key</span><code class="frc-code">export OPENAI_API_KEY=sk-...</code></div>
      <div class="frc-item ${testResult==='ok'?'frc-done':''}"><span class="frc-check">${testResult==='ok'?'\u2713':'\u25CB'}</span><span>Test your connection</span><button class="btn btn-sm ${testing?'':'primary'}" style="margin-left:8px" onClick=${testConnection} disabled=${testing||providerCount===0}>${testing?'Testing\u2026':testResult==='ok'?'\u2713 Connected':testResult==='fail'?'Retry':'Test connection'}</button>${testResult==='fail'?html`<span style="color:var(--red);font-size:0.78rem;margin-left:8px">Could not reach providers. Check your API keys.</span>`:null}</div>
      <div class="frc-item"><span class="frc-check">\u25CB</span><span>Send your first request</span><code class="frc-code">curl http://localhost:7749/v1/chat/completions -H "Content-Type: application/json" -H "Authorization: Bearer $OPENAI_API_KEY" -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}'</code></div>
      <div class="frc-item"><span class="frc-check">\u25CB</span><span>Point your app at the proxy</span><code class="frc-code">export OPENAI_BASE_URL=http://localhost:7749/v1</code></div>
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

