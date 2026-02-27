// ─── Proxy (Interactive toggles) ───────────────────────────────────
function ProxyView(){
  const[tab,setTab]=useState('modules');const[data,setData]=useState({});const[toast,setToast]=useState(null);const[filter,setFilter]=useState('');
  const load=async()=>{const[m,p,r]=await Promise.all([api('/api/proxy/modules'),api('/api/proxy/providers'),api('/api/proxy/routes')]);setData({modules:m.modules||[],providers:p.providers||[],routes:r.routes||[]})};
  useEffect(()=>{load()},[]);
  const toggleModule=async(name,enabled)=>{const r=await api('/api/proxy/modules/'+name,{method:'PUT',body:JSON.stringify({enabled:!enabled})});if(r._error){setToast({msg:'Failed: '+name,type:'error'});return}setToast({msg:name+' '+(enabled?'disabled':'enabled'),type:'success'});load()};
  const bulkToggle=async(cat,enabled)=>{const body=cat==='all'?{modules:d('modules').map(m=>m.name),enabled}:{category:cat,enabled};const r=await api('/api/proxy/modules/bulk',{method:'POST',body:JSON.stringify(body)});if(r._error){setToast({msg:'Bulk toggle failed',type:'error'});return}setToast({msg:(r.affected||0)+' modules '+(enabled?'enabled':'disabled'),type:'success'});load()};
  const d=k=>Array.isArray(data[k])?data[k]:[];
  const filtered=d('modules').filter(m=>!filter||m.name.toLowerCase().includes(filter.toLowerCase())||(m.category||'').toLowerCase().includes(filter.toLowerCase()));
  return html`<div class="page-head"><div class="page-eyebrow">Proxy</div><h2>Middleware Chain</h2><p class="page-sub">Toggle modules, manage providers & routes.</p></div>
    <div class="stats-row"><${Stat} label="Modules" value=${d('modules').length} sub=${d('modules').filter(m=>m.enabled).length+' enabled'} accent/><${Stat} label="In Chain" value=${d('modules').filter(m=>m.in_chain).length} sub="live middleware"/><${Stat} label="Providers" value=${d('providers').length}/><${Stat} label="Routes" value=${d('routes').length}/></div>
    <${TabBar} tabs=${['modules','chain','providers','routes']} active=${tab} onChange=${setTab}/>
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
    tab==='providers'?html`<${DataTable} columns=${[{key:'name',label:'Provider',width:'1fr',mono:true},{key:'status',label:'Status',width:'120px',render:r=>html`<${Badge} text=${r.status||'configured'} variant="success"/>`}]} rows=${d('providers')} emptyMsg="No providers configured."/>`:
    html`<${DataTable} columns=${[{key:'method',label:'Method',width:'100px',render:r=>html`<${Badge} text=${r.method||'ANY'} variant="muted"/>`},{key:'path',label:'Path',width:'1.5fr',mono:true},{key:'handler',label:'Handler',width:'1fr',mono:true}]} rows=${d('routes')} emptyMsg="No routes."/>`}
    ${toast&&html`<${Toast} msg=${toast.msg} type=${toast.type} onDone=${()=>setToast(null)}/>`}`;
}

