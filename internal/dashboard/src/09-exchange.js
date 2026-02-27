// ─── Exchange (with install/uninstall) ─────────────────────────────
function ExchangeView(){
  const[tab,setTab]=useState('packs');const[data,setData]=useState({});const[toast,setToast]=useState(null);const[installing,setInstalling]=useState(null);
  const[preview,setPreview]=useState(null);const[installResult,setInstallResult]=useState(null);
  const load=async()=>{const[p,i,st]=await Promise.all([api('/api/exchange/packs'),api('/api/exchange/installed'),api('/api/exchange/status')]);setData({packs:p.packs||[],installed:i.installed||[],status:st||{}})};
  useEffect(()=>{load()},[]);
  const previewPack=async(slug)=>{setPreview(null);const r=await api('/api/exchange/packs/'+slug+'/preview');setPreview(r)};
  const installPack=async(slug)=>{setInstalling(slug);setInstallResult(null);const r=await api('/api/exchange/packs/'+slug+'/install',{method:'POST'});setInstalling(null);if(r._error){setToast({msg:'Failed: '+(r.error||r._error),type:'error'});return}setInstallResult(r);setToast({msg:slug+' installed!',type:'success'});load()};
  const uninstall=async(id)=>{await api('/api/exchange/installed/'+id,{method:'DELETE'});setToast({msg:'Uninstalled',type:'success'});load()};
  const d=k=>Array.isArray(data[k])?data[k]:[];
  const installedSlugs=new Set(d('installed').map(i=>i.pack_slug));
  const st=data.status||{};
  const SECTION_ICONS={providers:'\uD83D\uDD0C',routes:'\u2194\uFE0F',modules:'\u2699\uFE0F',workflows:'\u26A1',tools:'\uD83D\uDD27',templates:'\uD83D\uDCDD',policies:'\uD83D\uDEE1\uFE0F',alerts:'\uD83D\uDD14'};
  return html`<div class="page-head"><div class="page-eyebrow">Exchange</div><h2>Pack Marketplace</h2><p class="page-sub">Install config packs to configure middleware, providers, policies, and more in one click.</p></div>
    <div class="stats-row"><${Stat} label="Available" value=${d('packs').length} accent/><${Stat} label="Installed" value=${d('installed').length}/><${Stat} label="Environments" value=${st.environments||0}/></div>
    <${TabBar} tabs=${['packs','installed']} active=${tab} onChange=${setTab}/>
    ${tab==='packs'?html`<div class="pack-grid">${d('packs').map(p=>{
      const isInstalled=installedSlugs.has(p.slug);
      return html`<div key=${p.slug} class="pack-card ${isInstalled?'pack-installed':''}">
      <div style="display:flex;justify-content:space-between;align-items:flex-start">
        <div class="pack-name">${p.name||p.slug}</div>
        ${isInstalled?html`<${Badge} text="installed" variant="success"/>`:null}
      </div>
      <div class="pack-desc">${p.description||'\u2014'}</div>
      <div class="pack-meta"><span class="mono" style="font-size:0.72rem">v${p.current_version||'1.0.0'}</span><span style="color:var(--cream-muted);font-size:0.72rem">${p.author||'unknown'}</span><span style="color:var(--cream-muted);font-size:0.72rem">${p.installs||0} installs</span></div>
      <div class="pack-actions" style="display:flex;gap:6px;margin-top:8px">
        <${Btn} small onClick=${()=>previewPack(p.slug)}>Preview<//> 
        ${isInstalled?null:html`<${Btn} small variant="primary" onClick=${()=>installPack(p.slug)} disabled=${installing===p.slug}>${installing===p.slug?'Installing\u2026':'Install'}<//>`}
      </div>
    </div>`})}</div>`:
    html`<${DataTable} columns=${[
      {key:'pack_slug',label:'Pack',width:'1fr',mono:true},
      {key:'version',label:'Version',width:'90px',mono:true},
      {key:'ts',label:'Installed',width:'130px',render:r=>fmt.ago(r.installed_at||r.created_at)},
      {key:'a',label:'',width:'100px',render:r=>html`<${Btn} small variant="danger" onClick=${()=>uninstall(r.id)}>Uninstall<//>`}
    ]} rows=${d('installed')} emptyMsg="No packs installed."/>`}
    ${preview&&html`<${Modal} title=${'Pack: '+preview.slug} onClose=${()=>setPreview(null)}>
      <div style="font-size:0.82rem">
        <div style="margin-bottom:10px;color:var(--cream-muted)">v${preview.version} \u2014 Installing will apply:</div>
        ${(preview.changes||[]).map(c=>html`<div key=${c.section} style="display:flex;align-items:center;gap:6px;padding:4px 0;border-bottom:1px solid var(--bg2)">
          <span>${SECTION_ICONS[c.section]||'\u2022'}</span>
          <span style="flex:1">${c.section}</span>
          <span class="mono" style="color:var(--green)">${c.count} item${c.count!==1?'s':''}</span>
        </div>`)}
        ${(preview.modules||[]).length>0?html`<div style="margin-top:10px">
          <div style="font-size:0.75rem;color:var(--cream-muted);margin-bottom:4px">Module changes:</div>
          ${preview.modules.map(m=>html`<div key=${m.name} style="display:flex;align-items:center;gap:6px;padding:2px 0;font-size:0.78rem">
            <span class="mono">${m.name}</span>
            <span style="margin-left:auto">${m.action==='unchanged'?html`<${Badge} text="no change" variant="muted"/>`:m.action==='update'?html`<${Badge} text=${'→ '+(m.enabled?'on':'off')} variant="warning"/>`:html`<${Badge} text="new" variant="success"/>`}</span>
          </div>`)}
        </div>`:''}
        <div style="margin-top:12px;display:flex;gap:8px">
          ${!installedSlugs.has(preview.slug)?html`<${Btn} small variant="primary" onClick=${()=>{setPreview(null);installPack(preview.slug)}}>Install Now<//>`:null}
          <${Btn} small onClick=${()=>setPreview(null)}>Close<//>
        </div>
      </div>
    <//>`}
    ${installResult&&html`<${Modal} title="Install Complete" onClose=${()=>setInstallResult(null)}>
      <div style="font-size:0.82rem">
        <div style="margin-bottom:8px"><${Badge} text=${installResult.slug+'@'+installResult.version} variant="success"/></div>
        ${Object.entries(installResult.applied||{}).map(([k,v])=>html`<div key=${k} style="padding:3px 0"><span style="color:var(--green)">\u2713</span> ${k}: <span class="mono">${v}</span> applied</div>`)}
        ${Object.entries(installResult.skipped||{}).map(([k,v])=>html`<div key=${k} style="padding:3px 0"><span style="color:var(--cream-muted)">\u2013</span> ${k}: <span class="mono">${v}</span> skipped (already exist)</div>`)}
        ${Object.entries(installResult.errors||{}).map(([k,errs])=>errs.map(e=>html`<div key=${k+e} style="padding:3px 0;color:var(--red)">\u2717 ${k}: ${e}</div>`))}
        <div style="margin-top:10px"><${Btn} small onClick=${()=>setInstallResult(null)}>Done<//></div>
      </div>
    <//>`}
    ${toast&&html`<${Toast} msg=${toast.msg} type=${toast.type} onDone=${()=>setToast(null)}/>`}`
}

