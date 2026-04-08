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
  const[rules,setRules]=useState([]);const[savings,setSavings]=useState(null);const[recommendations,setRecs]=useState([]);
  const[showAddRule,setShowAddRule]=useState(false);const[editRule,setEditRule]=useState(null);
  const[ruleForm,setRuleForm]=useState({name:'',priority:10,condField:'model',condOp:'==',condValue:'',actionModel:'',enabled:true});
  const load=async()=>{const[m,p,r]=await Promise.all([api('/api/proxy/modules'),api('/api/proxy/providers'),api('/api/proxy/routes')]);setData({modules:m.modules||[],providers:p.providers||[],routes:r.routes||[]})};
  const loadAliases=async()=>{const r=await api('/api/proxy/aliases');setAliases(r.aliases||[])};
  const loadRouting=async()=>{const[r,s,o]=await Promise.all([api('/api/proxy/routing/rules'),api('/api/proxy/routing/savings'),api('/api/proxy/routing/optimize')]);setRules(r.rules||[]);setSavings(s);setRecs(o.recommendations||[])};
  useEffect(()=>{load();loadAliases();loadRouting()},[]);
  const toggleModule=async(name,enabled)=>{const r=await api('/api/proxy/modules/'+name,{method:'PUT',body:JSON.stringify({enabled:!enabled})});if(r._error){setToast({msg:'Failed: '+name,type:'error'});return}setToast({msg:name+' '+(enabled?'disabled':'enabled'),type:'success'});load()};
  const bulkToggle=async(cat,enabled)=>{const body=cat==='all'?{modules:d('modules').map(m=>m.name),enabled}:{category:cat,enabled};const r=await api('/api/proxy/modules/bulk',{method:'POST',body:JSON.stringify(body)});if(r._error){setToast({msg:'Bulk toggle failed',type:'error'});return}setToast({msg:(r.affected||0)+' modules '+(enabled?'enabled':'disabled'),type:'success'});load()};
  const saveRule=async()=>{const body={name:ruleForm.name,priority:ruleForm.priority,condition:{field:ruleForm.condField,op:ruleForm.condOp,value:ruleForm.condValue},action:{route_to_model:ruleForm.actionModel},enabled:ruleForm.enabled};
    if(editRule){const r=await api('/api/proxy/routing/rules/'+editRule,{method:'PUT',body:JSON.stringify(body)});if(r._error){setToast({msg:'Update failed',type:'error'});return}setToast({msg:'Rule updated',type:'success'})}
    else{const r=await api('/api/proxy/routing/rules',{method:'POST',body:JSON.stringify(body)});if(r._error){setToast({msg:'Create failed',type:'error'});return}setToast({msg:'Rule created',type:'success'})}
    setShowAddRule(false);setEditRule(null);setRuleForm({name:'',priority:10,condField:'model',condOp:'==',condValue:'',actionModel:'',enabled:true});loadRouting()};
  const deleteRule=async id=>{await api('/api/proxy/routing/rules/'+id,{method:'DELETE'});setToast({msg:'Rule deleted',type:'success'});loadRouting()};
  const toggleRule=async(id,enabled)=>{await api('/api/proxy/routing/rules/'+id,{method:'PUT',body:JSON.stringify({enabled:!enabled})});loadRouting()};
  const startEditRule=r=>{setRuleForm({name:r.name,priority:r.priority,condField:r.condition?.field||'model',condOp:r.condition?.op||'==',condValue:r.condition?.value||'',actionModel:r.action?.route_to_model||'',enabled:r.enabled});setEditRule(r.id);setShowAddRule(true)};
  const applyRec=r=>{setRuleForm({name:r.suggested_rule.name,priority:r.suggested_rule.priority,condField:r.suggested_rule.condition.field,condOp:r.suggested_rule.condition.op,condValue:r.suggested_rule.condition.value,actionModel:r.suggested_rule.action.route_to_model,enabled:true});setEditRule(null);setShowAddRule(true);setTab('routing')};
  const addAlias=async()=>{if(!newAlias||!newModel)return;const r=await api('/api/proxy/aliases',{method:'PUT',body:JSON.stringify({alias:newAlias,model:newModel})});if(r._error){setToast({msg:'Failed: '+(r.error||''),type:'error'});return}setToast({msg:'Alias '+newAlias+' set',type:'success'});setNewAlias('');setNewModel('');loadAliases()};
  const deleteAlias=async name=>{await api('/api/proxy/aliases/'+name,{method:'DELETE'});setToast({msg:'Deleted '+name,type:'success'});loadAliases()};
  const exportAliases=()=>{const yaml='# stockyard aliases\naliases:\n'+aliases.map(a=>'  '+a.alias+': "'+a.model+'"').join('\n');navigator.clipboard.writeText(yaml);setToast({msg:'Copied '+aliases.length+' aliases as YAML',type:'success'})};
  const importAliases=async()=>{const input=prompt('Paste YAML aliases (one per line: alias: "model")');if(!input)return;const lines=input.split('\n').filter(l=>l.includes(':')&&!l.trim().startsWith('#')&&!l.trim().startsWith('aliases'));let count=0;for(const l of lines){const[a,...rest]=l.split(':');const alias=a.trim();const model=rest.join(':').trim().replace(/"/g,'');if(alias&&model){await api('/api/proxy/aliases',{method:'PUT',body:JSON.stringify({alias,model})});count++}}setToast({msg:'Imported '+count+' aliases',type:'success'});loadAliases()};
  const d=k=>Array.isArray(data[k])?data[k]:[];
  const filtered=d('modules').filter(m=>!filter||m.name.toLowerCase().includes(filter.toLowerCase())||(m.category||'').toLowerCase().includes(filter.toLowerCase()));
  return html`<div class="page-head"><div class="page-eyebrow">Chute</div><h2>Proxy & Middleware</h2><p class="page-sub">Toggle modules, manage providers, routes & aliases.</p></div>
    <div class="stats-row"><${Stat} label="Modules" value=${d('modules').length} sub=${d('modules').filter(m=>m.enabled).length+' enabled'} accent/><${Stat} label="In Chain" value=${d('modules').filter(m=>m.in_chain).length} sub="live middleware"/><${Stat} label="Providers" value=${d('providers').length}/><${Stat} label="Routes" value=${rules.length} sub=${rules.filter(r=>r.enabled).length+' active'}/><${Stat} label="Aliases" value=${aliases.length}/></div>
    <${TabBar} tabs=${['modules','chain','providers','routing','aliases','routes','diagnostics']} active=${tab} onChange=${setTab}/>
    ${tab==='modules'?html`<div style="margin-bottom:12px;display:flex;gap:12px;align-items:center"><input class="field-input" placeholder="Filter modules..." value=${filter} onInput=${e=>setFilter(e.target.value)} style="max-width:300px"/>
      <${Btn} small onClick=${()=>bulkToggle('all',false)} disabled=${!d('modules').some(m=>m.enabled)}>Disable All<//><${Btn} small onClick=${()=>bulkToggle('all',true)}>Enable All<//></div>
      <div class="data-table"><div class="dt-head" style="grid-template-columns:1.5fr 100px 80px 80px 80px"><span>Module</span><span>Category</span><span>Chain</span><span>Status</span><span>Toggle</span></div>
      <div class="dt-body">${filtered.map(m=>html`<div key=${m.name} class="dt-row" style="grid-template-columns:1.5fr 100px 80px 80px 80px">
        <span class="mono" data-label="Module">${m.name}</span><span class="mono" data-label="Category" style="font-size:0.72rem;color:var(--cream-muted)">${m.category||'general'}</span>
        <span data-label="Chain">${m.in_chain?html`<${Badge} text="live" variant="success"/>`:html`<${Badge} text="db" variant="muted"/>`}</span>
        <span data-label="Status"><${Badge} text=${m.enabled?'on':'off'} variant=${m.enabled?'success':'muted'}/></span>
        <span data-label="Toggle"><button class="toggle-btn ${m.enabled?'on':''}" onClick=${()=>toggleModule(m.name,m.enabled)}><span class="toggle-knob"></span></button></span>
      </div>`)}</div></div>`:
    tab==='chain'?html`<div class="data-table"><div class="dt-head" style="grid-template-columns:1.5fr 100px 100px 80px"><span>Middleware</span><span>Category</span><span>Status</span><span>Toggle</span></div>
      <div class="dt-body">${d('modules').filter(m=>m.in_chain).map(m=>html`<div key=${m.name} class="dt-row" style="grid-template-columns:1.5fr 100px 100px 80px">
        <span class="mono" data-label="Middleware">${m.name}</span><span class="mono" data-label="Category" style="font-size:0.72rem;color:var(--cream-muted)">${m.category||'general'}</span>
        <span data-label="Status"><${Badge} text=${m.enabled?'on':'off'} variant=${m.enabled?'success':'muted'}/></span>
        <span data-label="Toggle"><button class="toggle-btn ${m.enabled?'on':''}" onClick=${()=>toggleModule(m.name,m.enabled)}><span class="toggle-knob"></span></button></span>
      </div>`)}</div></div>`:
    tab==='providers'?html`<div class="stats-row" style="margin-bottom:12px"><${Stat} label="Configured" value=${d('providers').length} accent/><${Stat} label="Active" value=${d('providers').filter(p=>p.status==='active').length} sub="healthy"/><${Stat} label="Errors" value=${d('providers').reduce((s,p)=>s+(p.error_count||0),0)} sub=${d('providers').some(p=>p.error_count>0)?'check logs':'none'}/></div>
      <${DataTable} columns=${[{key:'name',label:'Provider',width:'1fr',mono:true},{key:'status',label:'Status',width:'120px',render:r=>html`<${Badge} text=${r.status||'configured'} variant=${r.status==='active'?'success':r.status==='error'?'danger':'muted'}/>`},{key:'errors',label:'Errors',width:'80px',mono:true,render:r=>r.error_count||0},{key:'reqs',label:'Requests',width:'100px',mono:true,render:r=>fmt.num(r.request_count||0)}]} rows=${d('providers')} emptyMsg="No providers configured. Set OPENAI_API_KEY or ANTHROPIC_API_KEY as environment variables and restart."/>`:
    tab==='routing'?html`<div>
      ${showAddRule?html`<div style="background:var(--bg2);border:1px solid var(--bg3);padding:16px;margin-bottom:16px">
        <div style="font-size:0.85rem;font-weight:bold;margin-bottom:12px;color:var(--cream)">${editRule?'Edit':'New'} Routing Rule</div>
        <div style="display:grid;grid-template-columns:1fr 80px;gap:8px;margin-bottom:8px">
          <input class="field-input" placeholder="Rule name" value=${ruleForm.name} onInput=${e=>setRuleForm({...ruleForm,name:e.target.value})}/>
          <input class="field-input" type="number" placeholder="Priority" value=${ruleForm.priority} onInput=${e=>setRuleForm({...ruleForm,priority:parseInt(e.target.value)||0})} style="text-align:center"/>
        </div>
        <div style="font-size:0.75rem;color:var(--cream-muted);margin-bottom:6px">When</div>
        <div style="display:grid;grid-template-columns:1fr 80px 1fr;gap:8px;margin-bottom:8px">
          <select class="field-input" value=${ruleForm.condField} onChange=${e=>setRuleForm({...ruleForm,condField:e.target.value})}>
            <option value="model">model</option><option value="provider">provider</option><option value="tokens_in">tokens_in</option><option value="user_id">user_id</option>
          </select>
          <select class="field-input" value=${ruleForm.condOp} onChange=${e=>setRuleForm({...ruleForm,condOp:e.target.value})}>
            <option value="==">==</option><option value="!=">!=</option><option value=">">></option><option value="<"><</option><option value="contains">contains</option>
          </select>
          <input class="field-input" placeholder="Value (e.g. gpt-4o)" value=${ruleForm.condValue} onInput=${e=>setRuleForm({...ruleForm,condValue:e.target.value})}/>
        </div>
        <div style="font-size:0.75rem;color:var(--cream-muted);margin-bottom:6px">Route to</div>
        <div style="display:grid;grid-template-columns:1fr auto;gap:8px;margin-bottom:12px">
          <input class="field-input" placeholder="Target model (e.g. gpt-4o-mini)" value=${ruleForm.actionModel} onInput=${e=>setRuleForm({...ruleForm,actionModel:e.target.value})}/>
          <label style="display:flex;align-items:center;gap:6px;font-size:0.8rem;color:var(--cream-dim);cursor:pointer">
            <input type="checkbox" checked=${ruleForm.enabled} onChange=${e=>setRuleForm({...ruleForm,enabled:e.target.checked})}/> Enabled
          </label>
        </div>
        <div style="display:flex;gap:8px">
          <${Btn} small variant="primary" onClick=${saveRule} disabled=${!ruleForm.name||!ruleForm.condValue||!ruleForm.actionModel}>${editRule?'Update':'Create'} Rule<//>
          <${Btn} small onClick=${()=>{setShowAddRule(false);setEditRule(null);setRuleForm({name:'',priority:10,condField:'model',condOp:'==',condValue:'',actionModel:'',enabled:true})}}>Cancel<//>
        </div>
      </div>`:html`<div style="margin-bottom:12px"><${Btn} small variant="primary" onClick=${()=>setShowAddRule(true)}>+ Add Routing Rule<//></div>`}
      ${savings&&savings.total_saved_usd>0?html`<div style="background:var(--bg2);border:1px solid var(--green);padding:12px;margin-bottom:12px;display:flex;align-items:center;gap:12px">
        <span style="font-size:1.4rem;color:var(--green)">\u2193</span>
        <div><div style="font-size:0.85rem;color:var(--green)">$${savings.total_saved_usd.toFixed(2)} saved this week</div>
        <div style="font-size:0.72rem;color:var(--cream-muted)">${savings.savings?.length||0} model substitutions active</div></div>
      </div>`:''}
      <${DataTable} columns=${[
        {key:'name',label:'Rule',width:'1.2fr',mono:true},
        {key:'priority',label:'Pri',width:'60px',mono:true,render:r=>r.priority},
        {key:'cond',label:'When',width:'1.5fr',mono:true,render:r=>{const c=r.condition||{};return (c.field||'?')+' '+(c.op||'?')+' '+(c.value||'?')}},
        {key:'action',label:'Route To',width:'1fr',mono:true,render:r=>r.action?.route_to_model||JSON.stringify(r.action||{})},
        {key:'enabled',label:'Status',width:'80px',render:r=>html`<button class="toggle-btn ${r.enabled?'on':''}" onClick=${()=>toggleRule(r.id,r.enabled)}><span class="toggle-knob"></span></button>`},
        {key:'actions',label:'',width:'120px',render:r=>html`<div style="display:flex;gap:4px"><${Btn} small onClick=${()=>startEditRule(r)}>Edit<//><${Btn} small variant="danger" onClick=${()=>deleteRule(r.id)}>Del<//></div>`}
      ]} rows=${rules} emptyMsg="No routing rules. Rules let you automatically route expensive models to cheaper alternatives based on conditions."/>
      ${recommendations.length>0?html`<div style="margin-top:16px">
        <div style="font-size:0.85rem;font-weight:bold;color:var(--cream);margin-bottom:8px">\u{1F4A1} Optimization Recommendations</div>
        ${recommendations.map(r=>html`<div style="background:var(--bg2);border:1px solid var(--bg3);padding:12px;margin-bottom:8px;display:flex;justify-content:space-between;align-items:center">
          <div>
            <div style="font-size:0.82rem;color:var(--cream)"><span class="mono">${r.current_model}</span> \u2192 <span class="mono">${r.suggested_model}</span></div>
            <div style="font-size:0.72rem;color:var(--cream-muted)">${r.request_count} requests/week \u2022 est. $${(r.estimated_monthly_savings||0).toFixed(2)}/mo savings</div>
          </div>
          <${Btn} small variant="primary" onClick=${()=>applyRec(r)}>Apply<//>
        </div>`)}
      </div>`:''}
    </div>`:
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
    tab==='routes'?html`<${DataTable} columns=${[{key:'method',label:'Method',width:'100px',render:r=>html`<${Badge} text=${r.method||'ANY'} variant="muted"/>`},{key:'path',label:'Path',width:'1.5fr',mono:true},{key:'handler',label:'Handler',width:'1fr',mono:true}]} rows=${d('routes')} emptyMsg="No routes."/>`:
    tab==='diagnostics'?html`<${ConfigDiagnostics} providers=${d('providers')} modules=${d('modules')} aliases=${aliases}/>`:
    null}
    ${toast&&html`<${Toast} msg=${toast.msg} type=${toast.type} onDone=${()=>setToast(null)}/>`}`;
}

