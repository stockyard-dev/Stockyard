// ─── Trust ─────────────────────────────────────────────────────────
function TrustView(){
  const[tab,setTab]=useState('safety');const[data,setData]=useState({});const[detail,setDetail]=useState(null);const[chainStatus,setChainStatus]=useState(null);const[verifying,setVerifying]=useState(false);
  const[evtFilter,setEvtFilter]=useState('all');const[autoRefresh,setAutoRefresh]=useState(false);const[showEvidenceForm,setShowEvidenceForm]=useState(false);
  const[safety,setSafety]=useState(null);const[safetyEvents,setSafetyEvents]=useState([]);
  const reload=async()=>{const[l,p,e,fb,st]=await Promise.all([api('/api/trust/ledger?limit=200'),api('/api/trust/policies'),api('/api/trust/evidence'),api('/api/trust/feedback'),api('/api/trust/status')]);setData({ledger:l.events||l.entries||[],policies:p.policies||[],evidence:e.packs||[],feedback:fb.feedback||[],status:st||{}})};
  const loadSafety=async()=>{const[s,e]=await Promise.all([api('/api/observe/safety/summary'),api('/api/observe/safety?limit=100')]);setSafety(s);setSafetyEvents(e.events||[])};
  useEffect(()=>{reload();loadSafety()},[]);
  useEffect(()=>{if(autoRefresh){const t=setInterval(()=>{reload();loadSafety()},5000);return()=>clearInterval(t)}},[autoRefresh]);
  const verifyChain=async()=>{setVerifying(true);const r=await api('/api/trust/ledger/verify');setChainStatus(r);setVerifying(false)};
  const createEvidence=async(name,from,to)=>{const r=await api('/api/trust/evidence',{method:'POST',body:JSON.stringify({name,date_from:from,date_to:to})});if(!r._error){setShowEvidenceForm(false);reload()}};
  const d=k=>Array.isArray(data[k])?data[k]:[];
  const st=data.status||{};const sf=safety||{};
  const EVENT_COLORS={'proxy_request':'var(--green)','spend_update':'var(--gold)','policy_violation':'var(--red)','admin_action':'var(--leather-light)','feedback':'var(--cream-muted)','system':'var(--rust-light)','forge_event':'var(--amber)','exchange_event':'var(--green)'};
  const evtTypes=[...new Set(d('ledger').map(e=>e.event_type))].sort();
  const filtered=evtFilter==='all'?d('ledger'):d('ledger').filter(e=>e.event_type===evtFilter);
  const typeCounts={};d('ledger').forEach(e=>{typeCounts[e.event_type]=(typeCounts[e.event_type]||0)+1});
  const scoreColor=s=>s>=80?'var(--green)':s>=50?'var(--gold)':'var(--red)';
  return html`<div class="page-head"><div class="page-eyebrow">Trust</div><h2>Safety & Compliance</h2><p class="page-sub">Safety scorecard, audit ledger, policies, evidence packs.</p></div>
    <div class="stats-row">
      <${Stat} label="Safety Score" value=${(sf.safety_score!=null?sf.safety_score+'%':'--')} accent sub=${'24h window'}/>
      <${Stat} label="Safety Events" value=${fmt.num(sf.total_events||0)} sub=${(sf.today?.blocked||0)+' blocked today'}/>
      <${Stat} label="Ledger Events" value=${fmt.num(st.ledger_events||d('ledger').length)} sub=${evtTypes.length+' types'}/>
      <${Stat} label="Chain" value=${chainStatus?chainStatus.valid?'\u2713 Valid':'\u2717 Broken':'Unverified'} sub=${chainStatus?chainStatus.events_checked+' checked':html`<button class="btn btn-sm" style="font-size:0.65rem;padding:2px 6px" onClick=${verifyChain}>${verifying?'Verifying\u2026':'Verify'}</button>`}/>
    </div>
    <${TabBar} tabs=${['safety','ledger','policies','evidence','feedback']} active=${tab} onChange=${setTab}/>
    ${tab==='safety'?html`<div class="safety-dash">
      <div class="safety-score-card">
        <div class="safety-score-ring" style="--score-color:${scoreColor(sf.safety_score||100)}">
          <div class="safety-score-num" style="color:${scoreColor(sf.safety_score||100)}">${sf.safety_score!=null?sf.safety_score:100}</div>
          <div style="font-size:0.72rem;color:var(--cream-muted)">Safety Score</div>
        </div>
        <div class="safety-score-detail">
          <div class="section-title" style="margin-bottom:8px">Active Defenses</div>
          <div style="display:flex;gap:16px;flex-wrap:wrap;margin-bottom:12px">
            <div class="safety-metric"><span class="safety-metric-val">${sf.active_modules||0}</span><span class="safety-metric-label">Safety Modules</span></div>
            <div class="safety-metric"><span class="safety-metric-val">${sf.active_policies||0}</span><span class="safety-metric-label">Trust Policies</span></div>
          </div>
          <div class="section-title" style="margin-bottom:8px">Today</div>
          <div style="display:flex;gap:16px;flex-wrap:wrap;margin-bottom:12px">
            <div class="safety-metric"><span class="safety-metric-val">${sf.today?.total||0}</span><span class="safety-metric-label">Events</span></div>
            <div class="safety-metric"><span class="safety-metric-val" style="color:var(--red)">${sf.today?.blocked||0}</span><span class="safety-metric-label">Blocked</span></div>
            <div class="safety-metric"><span class="safety-metric-val" style="color:var(--gold)">${sf.today?.redacted||0}</span><span class="safety-metric-label">Redacted</span></div>
          </div>
          <div class="section-title" style="margin-bottom:8px">Severity Breakdown</div>
          <div style="display:flex;gap:16px;flex-wrap:wrap">
            <div class="safety-metric"><span class="safety-metric-val" style="color:var(--red)">${sf.severity?.critical||0}</span><span class="safety-metric-label">Critical</span></div>
            <div class="safety-metric"><span class="safety-metric-val" style="color:var(--rust-light)">${sf.severity?.high||0}</span><span class="safety-metric-label">High</span></div>
            <div class="safety-metric"><span class="safety-metric-val" style="color:var(--gold)">${sf.severity?.medium||0}</span><span class="safety-metric-label">Medium</span></div>
            <div class="safety-metric"><span class="safety-metric-val" style="color:var(--cream-muted)">${sf.severity?.low||0}</span><span class="safety-metric-label">Low</span></div>
          </div>
        </div>
      </div>
      ${sf.by_type&&sf.by_type.length>0?html`<div style="margin-top:12px">
        <div class="section-title" style="margin-bottom:6px">Event Breakdown</div>
        <div style="display:flex;gap:6px;flex-wrap:wrap">${sf.by_type.map(t=>html`<div key=${t.event_type+t.severity} class="safety-type-pill">
          <span class="mono">${t.event_type}</span>
          <${Badge} text=${t.severity} variant=${t.severity==='critical'||t.severity==='high'?'danger':t.severity==='medium'?'warn':'muted'}/>
          <span class="mono" style="font-size:0.72rem">${t.action_taken}: ${t.count}</span>
        </div>`)}</div>
      </div>`:null}
      <div style="margin-top:14px">
        <div class="section-title" style="margin-bottom:6px">Recent Safety Events</div>
        <${DataTable} columns=${[
          {key:'ts',label:'Time',width:'100px',render:r=>fmt.ago(r.created_at)},
          {key:'type',label:'Event',width:'140px',mono:true,render:r=>html`<span style="color:${r.severity==='critical'?'var(--red)':r.severity==='high'?'var(--rust-light)':'var(--cream-muted)'}">${r.event_type}</span>`},
          {key:'sev',label:'Severity',width:'80px',render:r=>html`<${Badge} text=${r.severity} variant=${r.severity==='critical'||r.severity==='high'?'danger':r.severity==='medium'?'warn':'muted'}/>`},
          {key:'action',label:'Action',width:'80px',render:r=>html`<${Badge} text=${r.action_taken||'log'} variant=${r.action_taken==='block'?'danger':r.action_taken==='redact'?'warn':'muted'}/>`},
          {key:'cat',label:'Category',width:'100px',mono:true},
          {key:'model',label:'Model',width:'1fr',mono:true,render:r=>r.model||'\u2014'}
        ]} rows=${safetyEvents} emptyMsg="No safety events \u2014 events appear when PromptGuard, SecretScan, ToxicFilter or TrustEnforce fire."/>
      </div>
      <div style="margin-top:14px;padding:12px;background:var(--bg);border:1px solid var(--bg3);font-size:0.78rem;color:var(--cream-dim)">
        <div class="section-title" style="margin-bottom:6px">Safety Middleware Pipeline</div>
        <div style="display:flex;gap:4px;flex-wrap:wrap;align-items:center">
          ${['ipfence','ratelimit','promptguard','secretscan','tokentrim','toxicfilter','trust_enforce','compliancelog'].map((m,i)=>html`<span key=${m} class="safety-pipe-step">${m}</span>${i<7?html`<span style="color:var(--cream-muted)">\u2192</span>`:null}`)}
        </div>
        <div style="margin-top:6px;font-size:0.72rem;color:var(--cream-muted)">PII redaction \u2192 injection detection \u2192 secret scanning \u2192 toxicity filtering \u2192 policy enforcement \u2192 compliance logging</div>
      </div>
    </div>`:
    tab==='ledger'?html`<div style="display:flex;align-items:center;gap:8px;margin-bottom:8px;flex-wrap:wrap">
      <button class="btn btn-sm" onClick=${reload}>\u21BB Refresh</button>
      <button class="btn btn-sm ${autoRefresh?'primary':''}" onClick=${()=>setAutoRefresh(!autoRefresh)}>${autoRefresh?'\u23F8 Auto':'Auto \u25B6'}</button>
      <button class="btn btn-sm ${chainStatus?.valid?'primary':''}" onClick=${verifyChain}>${verifying?'Verifying\u2026':chainStatus?chainStatus.valid?'\u2713 Chain Valid':'\u2717 Broken #'+chainStatus.broken_at_id:'Verify Chain'}</button>
      <span style="border-left:1px solid var(--bg3);height:20px"></span>
      <select style="background:var(--bg2);color:var(--cream);border:1px solid var(--bg3);font-size:0.75rem;font-family:var(--font-mono);padding:3px 6px" onChange=${e=>setEvtFilter(e.target.value)} value=${evtFilter}>
        <option value="all">All types (${d('ledger').length})</option>
        ${evtTypes.map(t=>html`<option key=${t} value=${t}>${t} (${typeCounts[t]||0})</option>`)}
      </select>
      ${autoRefresh?html`<span style="display:inline-flex;align-items:center;gap:4px;font-size:0.72rem;color:var(--cream-muted)"><span style="width:6px;height:6px;border-radius:50%;background:var(--green);display:inline-block;animation:pulse 2s infinite"></span>live</span>`:''}
    </div>
    <${DataTable} columns=${[
      {key:'ts',label:'Time',width:'110px',render:r=>fmt.ago(r.created_at)},
      {key:'type',label:'Type',width:'130px',render:r=>html`<span style="display:inline-flex;align-items:center;gap:4px"><span style="width:6px;height:6px;border-radius:50%;background:${EVENT_COLORS[r.event_type]||'var(--cream-muted)'};display:inline-block"></span><span class="mono" style="font-size:0.78rem">${r.event_type}</span></span>`},
      {key:'action',label:'Action',width:'1fr',mono:true},
      {key:'actor',label:'Actor',width:'100px',mono:true},
      {key:'resource',label:'Resource',width:'1fr',mono:true,render:r=>html`<span style="max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;display:inline-block">${r.resource||'\u2014'}</span>`},
      {key:'hash',label:'Hash',width:'120px',mono:true,render:r=>fmt.hash(r.hash)}
    ]} rows=${filtered} onRowClick=${r=>setDetail(r)} emptyMsg="No audit events yet \u2014 ledger auto-records proxy requests, policy changes & admin actions."/>`:
    tab==='policies'?html`<${DataTable} columns=${[{key:'name',label:'Policy',width:'1.5fr'},{key:'type',label:'Type',width:'1fr',mono:true},{key:'config',label:'Config',width:'1.5fr',mono:true,render:r=>JSON.stringify(r.config||{}).substring(0,60)},{key:'enabled',label:'Status',width:'100px',render:r=>html`<${Badge} text=${r.enabled!==false?'active':'off'} variant=${r.enabled!==false?'success':'muted'}/>`}]} rows=${d('policies')} emptyMsg="No policies."/>`:
    tab==='evidence'?html`<div style="margin-bottom:10px">
      <button class="btn btn-sm primary" onClick=${()=>setShowEvidenceForm(!showEvidenceForm)}>${showEvidenceForm?'Cancel':'+ Create Evidence Pack'}</button>
    </div>
    ${showEvidenceForm?html`<${EvidenceForm} onCreate=${createEvidence}/>`:''} 
    <${DataTable} columns=${[{key:'name',label:'Pack',width:'1.5fr'},{key:'event_count',label:'Events',width:'90px',mono:true},{key:'range',label:'Range',width:'1.5fr',mono:true,render:r=>(r.date_from||'?')+' \u2192 '+(r.date_to||'?')},{key:'hash',label:'Hash',width:'120px',mono:true,render:r=>fmt.hash(r.hash)},{key:'status',label:'',width:'80px',render:r=>html`<${Badge} text=${r.status||'ok'} variant="success"/>`},{key:'ts',label:'Created',width:'110px',render:r=>fmt.ago(r.created_at)}]} rows=${d('evidence')} emptyMsg="No evidence packs \u2014 create one to snapshot the audit ledger."/>`:
    html`<${DataTable} columns=${[{key:'ts',label:'Time',width:'110px',render:r=>fmt.ago(r.created_at)},{key:'request_id',label:'Request',width:'1fr',mono:true},{key:'rating',label:'Rating',width:'80px',render:r=>'\u2605'.repeat(r.rating||0)+'\u2606'.repeat(5-(r.rating||0))},{key:'comment',label:'Comment',width:'2fr',render:r=>html`<span style="max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;display:inline-block">${r.comment||'\u2014'}</span>`},{key:'email',label:'User',width:'1fr',mono:true,render:r=>r.user_email||'\u2014'}]} rows=${d('feedback')} emptyMsg="No feedback submitted."/>`}
    ${detail&&html`<${Modal} title="Ledger Entry #${detail.id}" onClose=${()=>setDetail(null)}><div class="trace-detail">
      <div class="td-row"><span class="td-label">Type</span><span style="display:inline-flex;align-items:center;gap:6px"><span style="width:8px;height:8px;border-radius:50%;background:${EVENT_COLORS[detail.event_type]||'var(--cream-muted)'}"></span><span class="mono">${detail.event_type}</span></span></div>
      <div class="td-row"><span class="td-label">Action</span><span class="mono">${detail.action}</span></div>
      <div class="td-row"><span class="td-label">Actor</span><span class="mono">${detail.actor||'\u2014'}</span></div>
      <div class="td-row"><span class="td-label">Resource</span><span class="mono">${detail.resource||'\u2014'}</span></div>
      <div class="td-row"><span class="td-label">Time</span><span class="mono">${detail.created_at}</span></div>
      <div class="td-row"><span class="td-label">Hash</span><span class="mono" style="font-size:0.7rem;word-break:break-all">${detail.hash}</span></div>
      <div class="td-row"><span class="td-label">Prev Hash</span><span class="mono" style="font-size:0.7rem;word-break:break-all">${detail.prev_hash||'(genesis)'}</span></div>
      ${detail.detail?html`<div class="td-section"><div class="td-label">Detail</div><pre class="td-pre">${typeof detail.detail==='string'?detail.detail:JSON.stringify(detail.detail,null,2)}</pre></div>`:''}
    </div><//>}`
}
function EvidenceForm({onCreate}){
  const[name,setName]=useState('');const today=new Date().toISOString().split('T')[0];const[from,setFrom]=useState(today);const[to,setTo]=useState(today+'T23:59:59Z');
  return html`<div style="background:var(--bg);border:1px solid var(--bg3);padding:12px;margin-bottom:12px;display:flex;gap:8px;align-items:flex-end;flex-wrap:wrap">
    <div><label style="font-size:0.7rem;color:var(--cream-muted);display:block;margin-bottom:2px">Name</label><input value=${name} onInput=${e=>setName(e.target.value)} placeholder="Q1 Compliance Export" style="background:var(--bg2);color:var(--cream);border:1px solid var(--bg3);padding:4px 8px;font-size:0.8rem;width:200px"/></div>
    <div><label style="font-size:0.7rem;color:var(--cream-muted);display:block;margin-bottom:2px">From</label><input type="date" value=${from} onInput=${e=>setFrom(e.target.value)} style="background:var(--bg2);color:var(--cream);border:1px solid var(--bg3);padding:4px 8px;font-size:0.8rem"/></div>
    <div><label style="font-size:0.7rem;color:var(--cream-muted);display:block;margin-bottom:2px">To</label><input type="date" value=${to.split('T')[0]} onInput=${e=>setTo(e.target.value+'T23:59:59Z')} style="background:var(--bg2);color:var(--cream);border:1px solid var(--bg3);padding:4px 8px;font-size:0.8rem"/></div>
    <button class="btn btn-sm primary" disabled=${!name} onClick=${()=>onCreate(name,from,to)}>Generate</button>
  </div>`
}

