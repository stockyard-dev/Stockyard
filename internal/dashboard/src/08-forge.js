// ─── Forge ─────────────────────────────────────────────────────────
function ForgeView(){
  const[tab,setTab]=useState('workflows');const[data,setData]=useState({});const[toast,setToast]=useState(null);
  const[selWf,setSelWf]=useState(null);const[selRun,setSelRun]=useState(null);const[runInput,setRunInput]=useState('');const[running,setRunning]=useState(null);
  const load=async()=>{const[w,r,t]=await Promise.all([api('/api/forge/workflows'),api('/api/forge/runs'),api('/api/forge/tools')]);setData({workflows:w.workflows||[],runs:r.runs||[],tools:t.tools||[]})};
  useEffect(()=>{load()},[]);
  const viewWorkflow=async slug=>{const r=await api('/api/forge/workflows/'+slug);if(!r._error)setSelWf(r)};
  const runWorkflow=async slug=>{setRunning(slug);const inp=runInput||'Hello, test this workflow';const r=await api('/api/forge/workflows/'+slug+'/run',{method:'POST',body:JSON.stringify({input:inp})});setRunning(null);if(r._error){setToast({msg:'Failed: '+(r.error||r._error),type:'error'});return}setToast({msg:'Run started: '+r.run_id,type:'success'});setRunInput('');setSelWf(null);setTimeout(load,2000);setTimeout(load,5000)};
  const viewRun=async id=>{const[r,s]=await Promise.all([api('/api/forge/runs/'+id),api('/api/forge/runs/'+id+'/steps')]);if(!r._error){r._stepLogs=s.steps||[];setSelRun(r)}};
  const refreshRun=async()=>{if(!selRun)return;const[r,s]=await Promise.all([api('/api/forge/runs/'+selRun.id),api('/api/forge/runs/'+selRun.id+'/steps')]);if(!r._error){r._stepLogs=s.steps||[];setSelRun(r)}};
  useEffect(()=>{if(selRun&&(selRun.status==='running'||selRun.status==='pending')){const t=setInterval(()=>{refreshRun();load()},2000);return()=>clearInterval(t)}},[selRun?.id,selRun?.status]);
  const d=k=>Array.isArray(data[k])?data[k]:[];
  const STEP_COLORS={'llm':'var(--rust-light)','transform':'var(--gold)','tool':'var(--green)','http':'var(--leather-light)','gate':'var(--amber)'};
  return html`<div class="page-head"><div class="page-eyebrow">Forge</div><h2>Workflow Engine</h2><p class="page-sub">DAG workflows, tools & runs. 5 step types: LLM, transform, tool, HTTP, gate.</p></div>
    <div class="stats-row"><${Stat} label="Workflows" value=${d('workflows').length} accent/><${Stat} label="Runs" value=${d('runs').length} sub=${d('runs').filter(r=>r.status==='success').length+' succeeded'}/><${Stat} label="Tools" value=${d('tools').length}/></div>
    <${TabBar} tabs=${['workflows','runs','tools']} active=${tab} onChange=${setTab}/>
    ${tab==='workflows'?html`<div class="wf-grid">${d('workflows').map(w=>html`<div key=${w.slug} class="wf-card" onClick=${()=>viewWorkflow(w.slug)}>
      <div class="wf-card-head"><span class="wf-card-name">${w.name}</span><${Badge} text=${w.enabled!==false?'active':'off'} variant=${w.enabled!==false?'success':'muted'}/></div>
      <div class="wf-card-desc">${w.description||'\u2014'}</div>
      <div class="wf-card-meta"><span class="mono">${w.slug}</span><span class="mono">${w.step_count||0} steps</span><span class="mono">${w.trigger_type||'manual'}</span></div>
    </div>`)}</div>${d('workflows').length===0&&html`<div class="empty-state">No workflows. Install the Eval Suite pack or create one via API.</div>`}`:
    tab==='runs'?html`<${DataTable} columns=${[
      {key:'id',label:'Run',width:'140px',mono:true,render:r=>fmt.trunc(r.id,18)},
      {key:'wf',label:'Workflow',width:'120px',mono:true,render:r=>r.workflow_slug||'#'+r.workflow_id},
      {key:'status',label:'Status',width:'100px',render:r=>html`<${Badge} text=${r.status||'pending'} variant=${r.status==='success'?'success':r.status==='running'?'warn':r.status==='failed'?'danger':'muted'}/>`},
      {key:'progress',label:'Progress',width:'80px',mono:true,render:r=>(r.steps_completed||0)+'/'+(r.steps_total||0)},
      {key:'ts',label:'Started',width:'100px',render:r=>fmt.ago(r.started_at||r.created_at)},
      {key:'done',label:'Duration',width:'100px',mono:true,render:r=>{if(!r.completed_at||!r.started_at)return r.status==='running'?'running\u2026':'\u2014';const ms=new Date(r.completed_at)-new Date(r.started_at);return fmt.ms(ms)}}
    ]} rows=${d('runs')} onRowClick=${r=>viewRun(r.id)} emptyMsg="No runs yet \u2014 run a workflow to see results here."/>`:
    html`<${DataTable} columns=${[
      {key:'name',label:'Tool',width:'1.2fr',mono:true},
      {key:'type',label:'Type',width:'100px',render:r=>html`<${Badge} text=${r.type||'function'} variant=${r.type==='builtin'?'success':'muted'}/>`},
      {key:'desc',label:'Description',width:'2fr',render:r=>r.description||'\u2014'},
      {key:'enabled',label:'Status',width:'80px',render:r=>html`<${Badge} text=${r.enabled?'on':'off'} variant=${r.enabled?'success':'muted'}/>`}
    ]} rows=${d('tools')} emptyMsg="No tools. Tools will be seeded on next deploy."/>`}
    ${selWf&&html`<${Modal} title=${selWf.name||selWf.slug} onClose=${()=>setSelWf(null)}>
      <div class="wf-detail">
        <div style="font-size:0.85rem;color:var(--cream-dim);font-style:italic;margin-bottom:16px">${selWf.description||''}</div>
        <div class="section-title">Steps (${(selWf.steps||[]).length})</div>
        <div class="wf-steps">${(selWf.steps||[]).map((s,i)=>html`<div key=${s.id||i} class="wf-step">
          <div class="wf-step-head">
            <span class="wf-step-badge" style="background:${STEP_COLORS[s.type]||'var(--cream-muted)'}20;color:${STEP_COLORS[s.type]||'var(--cream-muted)'}">${s.type||'llm'}</span>
            <span class="wf-step-id">${s.id}</span>
            ${s.depends_on&&s.depends_on.length>0?html`<span class="wf-step-deps">\u2190 ${s.depends_on.join(', ')}</span>`:''}
          </div>
          ${s.config?.model?html`<div class="wf-step-field"><span class="wf-step-label">model</span><span class="mono">${s.config.model}</span></div>`:''}
          ${s.config?.expression?html`<div class="wf-step-field"><span class="wf-step-label">expr</span><span class="mono">${s.config.expression}</span></div>`:''}
          ${s.config?.tool_name?html`<div class="wf-step-field"><span class="wf-step-label">tool</span><span class="mono">${s.config.tool_name}</span></div>`:''}
          ${s.config?.condition?html`<div class="wf-step-field"><span class="wf-step-label">gate</span><span class="mono">${s.config.condition} ${s.config.threshold||''}</span></div>`:''}
          ${s.config?.prompt?html`<div class="wf-step-prompt">${fmt.trunc(s.config.prompt,120)}</div>`:''}
        </div>`)}</div>
        <div class="section-title" style="margin-top:20px">Run Workflow</div>
        <textarea class="field-input mono" rows="3" placeholder='Enter input (e.g. "What is quantum computing?")' value=${runInput} onInput=${e=>setRunInput(e.target.value)} style="resize:vertical;margin-bottom:12px;font-size:0.78rem"/>
        <div style="display:flex;gap:8px;justify-content:flex-end">
          <${Btn} onClick=${()=>setSelWf(null)}>Close<//>
          <${Btn} variant="primary" onClick=${()=>runWorkflow(selWf.slug)} disabled=${running===selWf.slug}>${running===selWf.slug?'Starting\u2026':'Run Workflow'}<//>
        </div>
      </div>
    <//>`}
    ${selRun&&html`<${Modal} title=${'Run: '+fmt.trunc(selRun.id,20)} onClose=${()=>setSelRun(null)}>
      <div class="wf-detail">
        <div class="td-row"><span class="td-label">Status</span><span><${Badge} text=${selRun.status} variant=${selRun.status==='success'?'success':selRun.status==='failed'?'danger':'warn'}/>${(selRun.status==='running'||selRun.status==='pending')?html` <${Btn} small onClick=${refreshRun}>\u21BB Refresh<//>`:''}</span></div>
        <div class="td-row"><span class="td-label">Workflow</span><span class="mono">${selRun.workflow_slug||'#'+selRun.workflow_id}</span></div>
        <div class="td-row"><span class="td-label">Progress</span><span class="mono">${selRun.steps_completed||0} / ${selRun.steps_total||0}</span></div>
        ${selRun.error?html`<div class="td-row"><span class="td-label">Error</span><span style="color:var(--red);font-size:0.82rem">${selRun.error}</span></div>`:''}
        ${selRun.input?html`<div class="td-section"><div class="td-label">Input</div><pre class="td-pre">${typeof selRun.input==='string'?selRun.input:JSON.stringify(selRun.input,null,2)}</pre></div>`:''}
        ${(selRun._stepLogs&&selRun._stepLogs.length>0)?html`<div class="td-section"><div class="td-label">Step Execution Log</div>
          ${selRun._stepLogs.map((s,i)=>html`<div key=${s.step_id||i} class="run-step-result">
            <div class="run-step-head">
              <span class="wf-step-badge" style="background:${STEP_COLORS[s.step_type]||'var(--cream-muted)'}20;color:${STEP_COLORS[s.step_type]||'var(--cream-muted)'}">${s.step_type||'llm'}</span>
              <span class="mono">${s.step_id}</span>
              <${Badge} text=${s.status||'?'} variant=${s.status==='success'?'success':s.status==='running'?'warn':s.status==='skipped'?'muted':'danger'}/>
              ${s.latency_ms?html`<span class="mono" style="font-size:0.7rem;color:var(--cream-muted)">${fmt.ms(s.latency_ms)}</span>`:''}
            </div>
            ${s.tokens_in||s.tokens_out?html`<div style="font-family:var(--font-mono);font-size:0.7rem;color:var(--cream-muted);margin:4px 0">${s.tokens_in||0} in / ${s.tokens_out||0} out</div>`:''}
            ${s.output?html`<pre class="td-pre" style="max-height:120px">${fmt.trunc(s.output,500)}</pre>`:''}
            ${s.error?html`<div style="color:var(--red);font-size:0.78rem;margin-top:4px">${s.error}</div>`:''}
          </div>`)}
        </div>`:selRun.output?html`<div class="td-section"><div class="td-label">Step Results</div>
          ${typeof selRun.output==='object'&&selRun.output!==null?Object.entries(selRun.output).map(([k,v])=>html`<div key=${k} class="run-step-result">
            <div class="run-step-head"><span class="mono">${k}</span><${Badge} text=${v.status||'?'} variant=${v.status==='success'?'success':'danger'}/>${v.latency_ms?html`<span class="mono" style="font-size:0.7rem;color:var(--cream-muted)">${fmt.ms(v.latency_ms)}</span>`:''}</div>
            ${v.tokens_in||v.tokens_out?html`<div style="font-family:var(--font-mono);font-size:0.7rem;color:var(--cream-muted);margin:4px 0">${v.tokens_in||0} in / ${v.tokens_out||0} out</div>`:''}
            ${v.output?html`<pre class="td-pre" style="max-height:120px">${fmt.trunc(v.output,500)}</pre>`:''}
            ${v.error?html`<div style="color:var(--red);font-size:0.78rem;margin-top:4px">${v.error}</div>`:''}
          </div>`):html`<pre class="td-pre">${JSON.stringify(selRun.output,null,2)}</pre>`}
        </div>`:''}
      </div>
    <//>`}
    ${toast&&html`<${Toast} msg=${toast.msg} type=${toast.type} onDone=${()=>setToast(null)}/>`}`;
}

