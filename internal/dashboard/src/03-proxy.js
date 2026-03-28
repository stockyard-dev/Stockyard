// ─── Config Diagnostics ────────────────────────────────────────────
function ConfigDiagnostics({providers,modules,aliases}){
  const checks=[];
  // Provider checks
  if(!providers||providers.length===0)checks.push({level:'warn',msg:'No providers configured. Set OPENAI_API_KEY or ANTHROPIC_API_KEY as environment variables.'});
  else checks.push({level:'ok',msg:providers.length+' provider'+(providers.length>1?'s':'')+' configured: '+providers.map(p=>p.name).join(', ')});
  const errProvs=providers?.filter(p=>p.error_count>0)||[];
  if(errProvs.length>0)checks.push({level:'warn',msg:'Provider errors detected: '+errProvs.map(p=>p.name+' ('+p.error_count+')').join(', ')});
  // Module checks
  const enabled=modules?.filter(m=>m.enabled)||[];
  if(enabled.length===0)checks.push({level:'info',msg:'No modules enabled. The proxy will forward requests without middleware processing.'});
  else checks.push({level:'ok',msg:enabled.length+' modules enabled.'});
  const hasFailover=enabled.some(m=>m.name==='failover');
  const hasCache=enabled.some(m=>m.name==='cachelayer'||m.name==='semanticcache');
  const hasCostcap=enabled.some(m=>m.name==='costcap');
  const hasRatelimit=enabled.some(m=>m.name==='rateshield');
  if(providers?.length>1&&!hasFailover)checks.push({level:'info',msg:'Multiple providers configured but failover is disabled. Enable the failover module for automatic provider fallback.'});
  if(!hasCache)checks.push({level:'info',msg:'Caching is disabled. Enable cachelayer to reduce costs on repeated prompts.'});
  if(!hasCostcap)checks.push({level:'info',msg:'No spending cap set. Enable costcap to prevent runaway costs.'});
  if(!hasRatelimit)checks.push({level:'info',msg:'Rate limiting is disabled. Enable rateshield to prevent request loops from draining your budget.'});
  // Alias checks
  if(aliases?.length>0)checks.push({level:'ok',msg:aliases.length+' alias'+(aliases.length>1?'es':'')+' configured.'});
  const icons={ok:'\u2713',warn:'\u26A0',info:'\u2139'};
  const colors={ok:'var(--green)',warn:'var(--gold)',info:'var(--cream-muted)'};
  return html`<div style="margin-top:8px">
    ${checks.map((c,i)=>html`<div key=${i} style="display:flex;align-items:flex-start;gap:10px;padding:10px 12px;border-bottom:1px solid var(--bg3)">
      <span style="color:${colors[c.level]};font-size:1rem;flex-shrink:0;width:20px;text-align:center">${icons[c.level]}</span>
      <span style="font-size:0.85rem;color:${c.level==='ok'?'var(--cream)':'var(--cream-dim)'}">${c.msg}</span>
    </div>`)}
    <div style="padding:16px 12px;font-size:0.78rem;color:var(--cream-muted);font-style:italic">Diagnostics check your current configuration for common issues. No external calls are made.</div>
  </div>`;
}

// ─── Proxy (Interactive toggles) ───────────────────────────────────
function ProxyView(){
  const[tab,setTab]=useState('modules');const[data,setData]=useState({});const[toast,setToast]=useState(null);const[filter,setFilter]=useState('');
  const[aliases,setAliases]=useState([]);const[newAlias,setNewAlias]=useState('');const[newModel,setNewModel]=useState('');
  const load=async()=>{const[m,p,r]=await Promise.all([api('/api/proxy/modules'),api('/api/proxy/providers'),api('/api/proxy/routes')]);setData({modules:m.modules||[],providers:p.providers||[],routes:r.routes||[]})};
  const loadAliases=async()=>{const r=await api('/api/proxy/aliases');setAliases(r.aliases||[])};
  useEffect(()=>{load();loadAliases()},[]);
  const toggleModule=async(name,enabled)=>{const r=await api('/api/proxy/modules/'+name,{method:'PUT',body:JSON.stringify({enabled:!enabled})});if(r._error){setToast({msg:'Failed: '+name,type:'error'});return}setToast({msg:name+' '+(enabled?'disabled':'enabled'),type:'success'});load()};
  const bulkToggle=async(cat,enabled)=>{const body=cat==='all'?{modules:d('modules').map(m=>m.name),enabled}:{category:cat,enabled};const r=await api('/api/proxy/modules/bulk',{method:'POST',body:JSON.stringify(body)});if(r._error){setToast({msg:'Bulk toggle failed',type:'error'});return}setToast({msg:(r.affected||0)+' modules '+(enabled?'enabled':'disabled'),type:'success'});load()};
  const addAlias=async()=>{if(!newAlias||!newModel)return;const r=await api('/api/proxy/aliases',{method:'PUT',body:JSON.stringify({alias:newAlias,model:newModel})});if(r._error){setToast({msg:'Failed: '+(r.error||''),type:'error'});return}setToast({msg:'Alias '+newAlias+' set',type:'success'});setNewAlias('');setNewModel('');loadAliases()};
  const deleteAlias=async name=>{await api('/api/proxy/aliases/'+name,{method:'DELETE'});setToast({msg:'Deleted '+name,type:'success'});loadAliases()};
  const exportAliases=()=>{const yaml='# stockyard aliases\naliases:\n'+aliases.map(a=>'  '+a.alias+': "'+a.model+'"').join('\n');navigator.clipboard.writeText(yaml);setToast({msg:'Copied '+aliases.length+' aliases as YAML',type:'success'})};
  const importAliases=()=>{const input=prompt('Paste YAML aliases (one per line: alias: "model")');if(!input)return;const lines=input.split('\n').filter(l=>l.includes(':')&&!l.trim().startsWith('#')&&!l.trim().startsWith('aliases'));let count=0;lines.forEach(async l=>{const[a,...rest]=l.split(':');const alias=a.trim();const model=rest.join(':').trim().replace(/"/g,'');if(alias&&model){await api('/api/proxy/aliases',{method:'PUT',body:JSON.stringify({alias,model})});count++}});setTimeout(()=>{setToast({msg:'Imported '+lines.length+' aliases',type:'success'});loadAliases()},500)};
  const d=k=>Array.isArray(data[k])?data[k]:[];
  const filtered=d('modules').filter(m=>!filter||m.name.toLowerCase().includes(filter.toLowerCase())||(m.category||'').toLowerCase().includes(filter.toLowerCase()));
  return html`<div class="page-head"><div class="page-eyebrow">Proxy</div><h2>Middleware Chain</h2><p class="page-sub">Toggle modules, manage providers, routes & aliases.</p></div>
    <div class="stats-row"><${Stat} label="Modules" value=${d('modules').length} sub=${d('modules').filter(m=>m.enabled).length+' enabled'} accent/><${Stat} label="In Chain" value=${d('modules').filter(m=>m.in_chain).length} sub="live middleware"/><${Stat} label="Providers" value=${d('providers').length}/><${Stat} label="Aliases" value=${aliases.length}/></div>
    <${TabBar} tabs=${['modules','chain','providers','aliases','routes','diagnostics']} active=${tab} onChange=${setTab}/>
    ${tab==='modules'?html`<div style="margin-bottom:12px;display:flex;gap:12px;align-items:center"><input class="field-input" placeholder="Filter modules..." value=${filter} onInput=${e=>setFilter(e.target.value)} style="max-width:300px"/>
      <${Btn} small onClick=${()=>bulkToggle('all',false)} disabled=${!d('modules').some(m=>m.enabled)}>Disable All<//><${Btn} small onClick=${()=>bulkToggle('all',true)}>Enable All<//></div>
      <div class="data-table"><div class="dt-head" style="grid-template-columns:1.5fr 100px 80px 80px 80px"><span>Module</span><span>Category</span><span>Chain</span><span>Status</span><span>Toggle</span></div>
      <div class="dt-body">${filtered.map(m=>html`<div key=${m.name} class="dt-row" style="grid-template-columns:1.5fr 100px 80px 80px 80px">
        <span class="mono">${m.name}</span><span class="mono" style="font-size:0.72rem;color:var(--cream-muted)">${m.category||'general'}</span>
        <span>${m.in_chain?html`<${Badge} text="live" variant="success"/>`:html`<${Badge} text="db" variant="muted"/>`}</span>
        <span><${Badge} text=${m.enabled?'on':'off'} variant=${m.enabled?'success':'muted'}/></span>
        <span><button class="toggle-btn ${m.enabled?'on':''}" onClick=${()=>toggleModule(m.name,m.enabled)}><span class="toggle-knob"></span></button></span>
      </div>`)}</div></div>`:
    tab==='chain'?html`<div class="data-table"><div class="dt-head" style="grid-template-columns:1.5fr 100px 100px 80px"><span>Middleware</span><span>Category</span><span>Status</span><span>Toggle</span></div>
      <div class="dt-body">${d('modules').filter(m=>m.in_chain).map(m=>html`<div key=${m.name} class="dt-row" style="grid-template-columns:1.5fr 100px 100px 80px">
        <span class="mono">${m.name}</span><span class="mono" style="font-size:0.72rem;color:var(--cream-muted)">${m.category||'general'}</span>
        <span><${Badge} text=${m.enabled?'on':'off'} variant=${m.enabled?'success':'muted'}/></span>
        <span><button class="toggle-btn ${m.enabled?'on':''}" onClick=${()=>toggleModule(m.name,m.enabled)}><span class="toggle-knob"></span></button></span>
      </div>`)}</div></div>`:
    tab==='providers'?html`<div class="stats-row" style="margin-bottom:12px"><${Stat} label="Configured" value=${d('providers').length} accent/><${Stat} label="Active" value=${d('providers').filter(p=>p.status==='active').length} sub="healthy"/><${Stat} label="Errors" value=${d('providers').reduce((s,p)=>s+(p.error_count||0),0)} sub=${d('providers').some(p=>p.error_count>0)?'check logs':'none'}/></div>
      <${DataTable} columns=${[{key:'name',label:'Provider',width:'1fr',mono:true},{key:'status',label:'Status',width:'120px',render:r=>html`<${Badge} text=${r.status||'configured'} variant=${r.status==='active'?'success':r.status==='error'?'danger':'muted'}/>`},{key:'errors',label:'Errors',width:'80px',mono:true,render:r=>r.error_count||0},{key:'reqs',label:'Requests',width:'100px',mono:true,render:r=>fmt.num(r.request_count||0)}]} rows=${d('providers')} emptyMsg="No providers configured. Set OPENAI_API_KEY or ANTHROPIC_API_KEY as environment variables and restart."/>`:
    tab==='aliases'?html`<div style="margin-bottom:12px;display:flex;gap:8px;align-items:center;flex-wrap:wrap">
      <input class="field-input" placeholder="Alias name" value=${newAlias} onInput=${e=>setNewAlias(e.target.value)} style="max-width:180px"/>
      <span class="mono" style="color:var(--cream-muted)">\u2192</span>
      <input class="field-input" placeholder="Model (e.g. gpt-4o)" value=${newModel} onInput=${e=>setNewModel(e.target.value)} style="max-width:240px"/>
      <${Btn} small variant="primary" onClick=${addAlias} disabled=${!newAlias||!newModel}>Add Alias<//>
      <span style="flex:1"></span>
      <${Btn} small onClick=${exportAliases} disabled=${aliases.length===0}>\u{1F4CB} Export YAML<//>
      <${Btn} small onClick=${importAliases}>\u{1F4E5} Import YAML<//>
    </div>
    <${DataTable} columns=${[{key:'alias',label:'Alias',width:'1fr',mono:true},{key:'model',label:'Model',width:'1.5fr',mono:true},{key:'a',label:'',width:'80px',render:r=>html`<${Btn} small variant="danger" onClick=${()=>deleteAlias(r.alias)}>Delete<//>`}]} rows=${aliases} emptyMsg="No aliases configured. Aliases let your app call logical names like 'primary-chat' instead of real model names."/>`:
    html`<${DataTable} columns=${[{key:'method',label:'Method',width:'100px',render:r=>html`<${Badge} text=${r.method||'ANY'} variant="muted"/>`},{key:'path',label:'Path',width:'1.5fr',mono:true},{key:'handler',label:'Handler',width:'1fr',mono:true}]} rows=${d('routes')} emptyMsg="No routes."/>`:
    tab==='diagnostics'?html`<${ConfigDiagnostics} providers=${d('providers')} modules=${d('modules')} aliases=${aliases}/>`:
    null}
    ${toast&&html`<${Toast} msg=${toast.msg} type=${toast.type} onDone=${()=>setToast(null)}/>`}`;
}

