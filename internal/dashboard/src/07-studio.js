// ─── Studio ────────────────────────────────────────────────────────
function StudioView(){
  const[tab,setTab]=useState('playground');const[data,setData]=useState({});const[toast,setToast]=useState(null);
  // Playground state
  const[prompt,setPrompt]=useState('');const[system,setSystem]=useState('');const[models,setModels]=useState(['gpt-4o-mini']);
  const[running,setRunning]=useState(false);const[pgResult,setPgResult]=useState(null);const[compareMode,setCompareMode]=useState(false);
  const[expResult,setExpResult]=useState(null);const[selExp,setSelExp]=useState(null);
  const MODEL_LIST=['gpt-4o-mini','gpt-4o','gpt-4.1-mini','gpt-4.1','claude-sonnet-4-5-20250929','claude-haiku-4-5-20251001','gemini-2.5-flash','deepseek-chat','mistral-small-latest','llama-3.3-70b-versatile'];
  const load=async()=>{const[t,e,b]=await Promise.all([api('/api/studio/templates'),api('/api/studio/experiments'),api('/api/studio/benchmarks')]);setData({templates:t.templates||[],experiments:e.experiments||[],benchmarks:b.benchmarks||[]})};
  useEffect(()=>{load()},[]);
  const d=k=>Array.isArray(data[k])?data[k]:[];
  const runPlayground=async()=>{
    if(!prompt.trim())return;setRunning(true);setPgResult(null);setExpResult(null);
    if(compareMode&&models.length>=2){
      const r=await api('/api/studio/experiments/run',{method:'POST',body:JSON.stringify({name:'playground-'+Date.now(),prompt,system,models,runs:1,eval:'length'})});
      setRunning(false);if(r._error||r.error){setToast({msg:r.error||r._error,type:'error'});return}setExpResult(r);load();
    }else{
      const r=await api('/api/studio/playground',{method:'POST',body:JSON.stringify({prompt,system,model:models[0]})});
      setRunning(false);if(r._error){setToast({msg:r.error||r._error,type:'error'});return}setPgResult(r);
    }
  };
  const loadTemplate=async(slug)=>{const t=await api('/api/studio/templates/'+slug);if(!t._error){setPrompt(t.content||'');if(t.model)setModels([t.model]);setSystem('');setTab('playground');setToast({msg:'Loaded: '+t.name,type:'success'})}};
  const toggleModel=(m)=>{if(models.includes(m)){if(models.length>1)setModels(models.filter(x=>x!==m))}else{setModels([...models,m])}};
  const viewExp=async(id)=>{const r=await api('/api/studio/experiments/'+id);if(!r._error)setSelExp(r)};
  return html`<div class="page-head"><div class="page-eyebrow">Studio</div><h2>Prompt Playground</h2><p class="page-sub">Test prompts, compare models side-by-side, run experiments.</p></div>
    <div class="stats-row"><${Stat} label="Templates" value=${d('templates').length} accent/><${Stat} label="Experiments" value=${d('experiments').length}/><${Stat} label="Benchmarks" value=${d('benchmarks').length}/></div>
    <${TabBar} tabs=${['playground','templates','experiments','benchmarks']} active=${tab} onChange=${setTab}/>
    ${tab==='playground'?html`<div class="pg-layout">
      <div class="pg-input">
        <div class="section-title" style="margin-bottom:6px">System Message <span style="color:var(--cream-muted);font-weight:400;font-size:0.72rem">(optional)</span></div>
        <textarea class="field-input mono" rows="2" placeholder="You are a helpful assistant..." value=${system} onInput=${e=>setSystem(e.target.value)} style="resize:vertical;font-size:0.78rem;margin-bottom:10px"/>
        <div class="section-title" style="margin-bottom:6px">Prompt</div>
        <textarea class="field-input mono" rows="5" placeholder="Type your prompt here..." value=${prompt} onInput=${e=>setPrompt(e.target.value)} style="resize:vertical;font-size:0.78rem;margin-bottom:10px"/>
        <div class="section-title" style="margin-bottom:6px;display:flex;align-items:center;gap:8px">
          Models
          <label style="font-size:0.72rem;color:var(--cream-muted);display:flex;align-items:center;gap:4px;cursor:pointer;font-weight:400">
            <input type="checkbox" checked=${compareMode} onChange=${e=>setCompareMode(e.target.checked)} style="accent-color:var(--rust-light)"/>
            Compare mode
          </label>
        </div>
        <div style="display:flex;gap:6px;flex-wrap:wrap;margin-bottom:12px">
          ${MODEL_LIST.map(m=>html`<button key=${m} class="btn btn-sm ${models.includes(m)?'primary':''}" 
            onClick=${()=>compareMode?toggleModel(m):setModels([m])}
            style="font-size:0.72rem;font-family:var(--font-mono);padding:3px 8px;${models.includes(m)?'':'opacity:0.6'}">${m}</button>`)}
        </div>
        <div style="display:flex;gap:8px;align-items:center">
          <${Btn} variant="primary" onClick=${runPlayground} disabled=${running||!prompt.trim()}>${running?(compareMode?'Comparing\u2026':'Running\u2026'):(compareMode?'Compare '+models.length+' Models':'Run')}<//>
          ${models.length<2&&compareMode?html`<span style="font-size:0.72rem;color:var(--cream-muted)">Select 2+ models to compare</span>`:''}
        </div>
      </div>
      <div class="pg-output">
        ${pgResult?html`<div class="pg-result-card">
          <div class="pg-result-head">
            <span class="mono" style="font-weight:600">${pgResult.model}</span>
            <span class="mono" style="font-size:0.72rem;color:var(--cream-muted)">${fmt.ms(pgResult.latency_ms)}</span>
            <span class="mono" style="font-size:0.72rem;color:var(--cream-muted)">${pgResult.tokens_in||0}\u2192${pgResult.tokens_out||0} tok</span>
            <span class="mono" style="font-size:0.72rem;color:var(--green)">$${(pgResult.cost_usd||0).toFixed(4)}</span>
          </div>
          ${pgResult.error?html`<div style="color:var(--red);font-size:0.82rem;padding:8px">${pgResult.error}</div>`
           :html`<div class="pg-result-body">${pgResult.content||'(empty response)'}</div>`}
        </div>`:
        expResult?html`<div class="pg-compare">
          ${(expResult.variants||[]).map(v=>html`<div key=${v.model} class="pg-result-card ${v.model===expResult.winner?'pg-winner':''}">
            <div class="pg-result-head">
              <span class="mono" style="font-weight:600">${v.model}</span>
              ${v.model===expResult.winner?html`<${Badge} text="winner" variant="success"/>`:null}
            </div>
            <div style="display:flex;gap:12px;padding:6px 10px;font-size:0.72rem;font-family:var(--font-mono);color:var(--cream-muted);border-bottom:1px solid var(--bg3)">
              <span>${fmt.ms(v.avg_latency_ms)}</span>
              <span>${v.avg_tokens_in||0}\u2192${v.avg_tokens_out||0} tok</span>
              <span style="color:var(--green)">$${(v.avg_cost_usd||0).toFixed(4)}</span>
              ${v.errors>0?html`<span style="color:var(--red)">${v.errors} errors</span>`:null}
            </div>
            ${v.runs&&v.runs[0]?html`<div class="pg-result-body">${v.runs[0].error?html`<span style="color:var(--red)">${v.runs[0].error}</span>`:v.runs[0].content||'(empty)'}</div>`:null}
          </div>`)}
          <div style="padding:8px;font-size:0.78rem;color:var(--cream-muted);text-align:center">
            Winner: <span class="mono" style="color:var(--green)">${expResult.winner||'none'}</span> by ${expResult.win_reason||'eval'} \u2014
            Total: $${(expResult.total_cost_usd||0).toFixed(4)} in ${fmt.ms(expResult.duration_ms)}
            ${expResult.experiment_id?html` \u2014 <span class="mono">exp #${expResult.experiment_id}</span>`:''}
          </div>
        </div>`:
        html`<div style="color:var(--cream-muted);text-align:center;padding:40px;font-size:0.85rem">
          <div style="font-size:1.5rem;margin-bottom:10px">\u26A1</div>
          <div>Type a prompt and hit Run to test it through the proxy.</div>
          <div style="margin-top:6px;font-size:0.78rem">Enable <strong>Compare mode</strong> to test multiple models side-by-side.</div>
          <div style="margin-top:6px;font-size:0.72rem;color:var(--cream-muted)">Requires API keys configured (OPENAI_API_KEY, etc.)</div>
        </div>`}
      </div>
    </div>`:
    tab==='templates'?html`<div class="pack-grid">${d('templates').map(t=>html`<div key=${t.slug} class="pack-card" style="cursor:pointer" onClick=${()=>loadTemplate(t.slug)}>
      <div class="pack-name">${t.name||t.slug}</div>
      <div class="pack-desc">${t.description||'\u2014'}</div>
      <div class="pack-meta"><span class="mono">${t.slug}</span>${t.model?html`<span class="mono">${t.model}</span>`:null}<span class="mono">v${t.current_version||1}</span></div>
      <div style="margin-top:6px"><${Btn} small onClick=${e=>{e.stopPropagation();loadTemplate(t.slug)}}>Load in Playground<//></div>
    </div>`)}</div>${d('templates').length===0?html`<div class="empty-state">No templates. Install the Dev Productivity pack to get starter templates.</div>`:''}`:
    tab==='experiments'?html`<${DataTable} columns=${[
      {key:'name',label:'Experiment',width:'1.5fr',mono:true},
      {key:'status',label:'Status',width:'100px',render:r=>html`<${Badge} text=${r.status||'draft'} variant=${r.status==='completed'?'success':r.status==='running'?'warn':'muted'}/>`},
      {key:'type',label:'Type',width:'100px',mono:true},
      {key:'ts',label:'Created',width:'110px',render:r=>fmt.ago(r.created_at)}
    ]} rows=${d('experiments')} onRowClick=${r=>viewExp(r.id)} emptyMsg="No experiments yet \u2014 run a comparison in the playground."/>`:
    html`<${DataTable} columns=${[{key:'name',label:'Benchmark',width:'1.5fr'},{key:'status',label:'Status',width:'100px',render:r=>html`<${Badge} text=${r.status||'pending'} variant=${r.status==='completed'?'success':'muted'}/>`},{key:'ts',label:'Created',width:'110px',render:r=>fmt.ago(r.created_at)}]} rows=${d('benchmarks')} emptyMsg="No benchmarks."/>`}
    ${selExp&&html`<${Modal} title=${'Experiment: '+(selExp.name||'#'+selExp.id)} onClose=${()=>setSelExp(null)}>
      <div class="trace-detail">
        <div class="td-row"><span class="td-label">Status</span><${Badge} text=${selExp.status} variant=${selExp.status==='completed'?'success':'muted'}/></div>
        <div class="td-row"><span class="td-label">Type</span><span class="mono">${selExp.type}</span></div>
        ${selExp.config?.prompt?html`<div class="td-section"><div class="td-label">Prompt</div><pre class="td-pre">${selExp.config.prompt}</pre></div>`:''}
        ${selExp.results?.variants?html`<div class="td-section"><div class="td-label">Results</div>
          ${selExp.results.variants.map(v=>html`<div key=${v.model} style="padding:6px 0;border-bottom:1px solid var(--bg2)">
            <div style="display:flex;gap:8px;align-items:center">
              <span class="mono" style="font-weight:600">${v.model}</span>
              ${v.model===selExp.results?.winner?html`<${Badge} text="winner" variant="success"/>`:null}
              <span class="mono" style="font-size:0.72rem;color:var(--cream-muted)">${fmt.ms(v.avg_latency_ms)} \u2014 $${(v.avg_cost_usd||0).toFixed(4)}</span>
            </div>
            ${v.runs&&v.runs[0]?html`<pre class="td-pre" style="max-height:100px;margin-top:4px">${fmt.trunc(v.runs[0].content||v.runs[0].error||'',300)}</pre>`:null}
          </div>`)}
        </div>`:''}
      </div>
    <//>`}
    ${toast&&html`<${Toast} msg=${toast.msg} type=${toast.type} onDone=${()=>setToast(null)}/>`}`
}

