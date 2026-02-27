// ─── Observe (with charts + trace detail) ──────────────────────────
function ObserveView(){
  const[tab,setTab]=useState('live');const[data,setData]=useState({});const[detail,setDetail]=useState(null);const[ts,setTs]=useState({});const[period,setPeriod]=useState('24h');
  const[liveEvents,setLiveEvents]=useState([]);const[sseStatus,setSseStatus]=useState('connecting');const[livePaused,setLivePaused]=useState(false);
  const[safety,setSafety]=useState(null);const[safetyEvents,setSafetyEvents]=useState([]);
  const liveRef=useRef([]);const maxLive=200;

  // SSE connection
  useEffect(()=>{
    const es=new EventSource('/ui/events');
    es.onopen=()=>setSseStatus('connected');
    es.onerror=()=>setSseStatus('disconnected');
    es.onmessage=(e)=>{
      try{
        const evt=JSON.parse(e.data);
        if(evt.type==='connected'){setSseStatus('connected');return}
        if(evt.type==='request_logged'||evt.type==='spend_update'){
          evt._ts=new Date().toISOString();evt._id=Math.random().toString(36).slice(2,10);
          liveRef.current=[evt,...liveRef.current].slice(0,maxLive);
          if(!livePaused)setLiveEvents([...liveRef.current]);
        }
      }catch(err){}
    };
    return()=>es.close();
  },[]);

  // Refresh live events display when unpaused
  useEffect(()=>{if(!livePaused)setLiveEvents([...liveRef.current])},[livePaused]);

  // Load persisted data
  const reload=async()=>{const[ov,t,c,al,an]=await Promise.all([api('/api/observe/overview'),api('/api/observe/traces'),api('/api/observe/costs'),api('/api/observe/alerts'),api('/api/observe/anomalies')]);setData({overview:ov||{},traces:t.traces||[],costs:c.costs||c.providers||[],alerts:al.rules||al.alerts||[],anomalies:an.anomalies||[]});const[ss,se]=await Promise.all([api('/api/observe/safety/summary'),api('/api/observe/safety?limit=50')]);setSafety(ss||{});setSafetyEvents(se.events||[])};
  useEffect(()=>{reload()},[]);
  useEffect(()=>{(async()=>{const r=await api('/api/observe/timeseries?period='+period);setTs(r||{})})()},[period]);
  // Auto-refresh persisted data every 15s when on live tab
  useEffect(()=>{if(tab==='live'){const t=setInterval(reload,15000);return()=>clearInterval(t)}},[tab]);

  const d=k=>Array.isArray(data[k])?data[k]:[];
  const ov=data.overview||{};const today=ov.today||{};
  const buckets=ts.buckets||[];const provs=ts.providers||[];const models=ts.models||[];
  const PCOLORS=['var(--rust-light)','var(--gold)','var(--green)','var(--leather-light)','var(--cream-muted)'];
  const sseDot=sseStatus==='connected'?'var(--green)':sseStatus==='connecting'?'var(--gold)':'var(--red)';

  return html`<div class="page-head"><div class="page-eyebrow">Observe</div><h2>Analytics & Traces</h2><p class="page-sub">Real-time streaming, cost attribution, alerts & anomaly detection.</p></div>
    <div class="stats-row">
      <${Stat} label="Today\u2019s Requests" value=${fmt.num(today.requests||0)} accent/>
      <${Stat} label="Today\u2019s Cost" value=${fmt.usd(today.cost_usd||0)}/>
      <${Stat} label="Total Traces" value=${fmt.num(ov.total_traces||0)}/>
      <${Stat} label="Live Feed" value=${liveEvents.length} sub=${html`<span style="display:inline-flex;align-items:center;gap:4px"><span style="width:6px;height:6px;border-radius:50%;background:${sseDot};display:inline-block;${sseStatus==='connected'?'animation:pulse 2s infinite':''}"></span>${sseStatus}</span>`}/>
    </div>
    <${TabBar} tabs=${['\u25CF live','dashboard','traces','costs','alerts','safety']} active=${tab==='live'?'\u25CF live':tab} onChange=${t=>setTab(t==='\u25CF live'?'live':t)}/>
    ${tab==='live'?html`
      <div style="display:flex;align-items:center;gap:8px;margin-bottom:12px">
        <button class="btn btn-sm ${livePaused?'':'primary'}" onClick=${()=>setLivePaused(!livePaused)}>${livePaused?'\u25B6 Resume':'\u23F8 Pause'}</button>
        <button class="btn btn-sm" onClick=${()=>{liveRef.current=[];setLiveEvents([])}}>Clear</button>
        <span class="mono" style="font-size:0.75rem;color:var(--cream-muted)">${liveEvents.length} events${livePaused?' (paused)':''}</span>
      </div>
      ${liveEvents.length===0?html`<div class="empty-state">
        <div style="font-size:1.5rem;margin-bottom:8px">\u{1F4E1}</div>
        <div>Waiting for live events\u2026</div>
        <div style="font-size:0.8rem;color:var(--cream-muted);margin-top:4px">Send requests to <span class="mono">/v1/chat/completions</span> to see them stream here in real-time.</div>
      </div>`:html`<div class="live-feed">${liveEvents.map(evt=>html`<div key=${evt._id} class="live-event ${evt.type==='spend_update'?'live-spend':'live-trace'}">
        <div class="live-event-head">
          <span style="width:6px;height:6px;border-radius:50%;background:${evt.type==='request_logged'?'var(--green)':'var(--gold)'};display:inline-block"></span>
          <span class="mono" style="font-size:0.78rem;color:var(--cream)">${evt.type==='request_logged'?'request':'spend'}</span>
          ${evt.model?html`<span class="mono" style="font-size:0.78rem">${evt.model}</span>`:''}
          ${evt.tokens?html`<span class="mono" style="font-size:0.72rem;color:var(--cream-muted)">${fmt.num(evt.tokens)} tok</span>`:''}
          ${evt.cost?html`<span class="mono" style="font-size:0.72rem;color:var(--gold)">${fmt.usd(evt.cost)}</span>`:''}
          ${evt.latency?html`<span class="mono" style="font-size:0.72rem;color:var(--cream-muted)">${fmt.ms(evt.latency)}</span>`:''}
          ${evt.cache_hit?html`<${Badge} text="cache" variant="success"/>`:''} 
          <span class="mono" style="font-size:0.68rem;color:var(--cream-muted);margin-left:auto">${evt._ts?new Date(evt._ts).toLocaleTimeString():''}</span>
        </div>
        ${evt.type==='spend_update'?html`<div style="font-size:0.75rem;color:var(--cream-muted);padding:2px 0 0 16px">project: ${evt.project||'\u2014'} \u2022 today: ${fmt.usd(evt.today||0)} \u2022 month: ${fmt.usd(evt.month||0)}</div>`:''}
      </div>`)}</div>`}`:
    tab==='dashboard'?html`<div class="chart-period"><span class="chart-period-label">Period:</span>${['24h','7d','30d'].map(p=>html`<button key=${p} class="btn btn-sm ${period===p?'primary':''}" onClick=${()=>setPeriod(p)}>${p}</button>`)}</div>
      <div class="chart-grid">
        <div class="chart-card"><div class="chart-title">Requests</div><${Sparkline} data=${buckets.map(b=>b.requests)} width=${280} height=${50}/></div>
        <div class="chart-card"><div class="chart-title">Cost (USD)</div><${Sparkline} data=${buckets.map(b=>b.cost_usd)} width=${280} height=${50} color="var(--gold)"/></div>
        <div class="chart-card"><div class="chart-title">Avg Latency (ms)</div><${Sparkline} data=${buckets.map(b=>b.avg_latency_ms)} width=${280} height=${50} color="var(--leather-light)"/></div>
        <div class="chart-card"><div class="chart-title">Errors</div><${Sparkline} data=${buckets.map(b=>b.errors)} width=${280} height=${50} color="var(--red)"/></div>
      </div>
      <div class="chart-grid" style="grid-template-columns:1fr 1fr">
        <div class="chart-card"><div class="chart-title">Cost by Provider (7d)</div><${MiniBar} items=${provs.map((p,i)=>({label:p.provider||'unknown',value:p.cost_usd,color:PCOLORS[i%PCOLORS.length],display:fmt.usd(p.cost_usd)}))} width=${340} height=${140}/></div>
        <div class="chart-card"><div class="chart-title">Cost by Model (7d)</div><${MiniBar} items=${models.map((m,i)=>({label:(m.model||'unknown').split('/').pop().substring(0,16),value:m.cost_usd,color:PCOLORS[i%PCOLORS.length],display:fmt.usd(m.cost_usd)}))} width=${340} height=${140}/></div>
      </div>`:
    tab==='traces'?html`<div style="margin-bottom:8px"><button class="btn btn-sm" onClick=${reload}>\u21BB Refresh</button></div><${DataTable} columns=${[
      {key:'ts',label:'Time',width:'100px',render:r=>fmt.ago(r.timestamp||r.created_at)},{key:'model',label:'Model',width:'1fr',mono:true},{key:'provider',label:'Provider',width:'110px',mono:true},
      {key:'ti',label:'In',width:'70px',mono:true,render:r=>fmt.num(r.tokens_in)},{key:'to',label:'Out',width:'70px',mono:true,render:r=>fmt.num(r.tokens_out)},
      {key:'lat',label:'Latency',width:'90px',mono:true,render:r=>fmt.ms(r.duration_ms||r.latency_ms)},{key:'cost',label:'Cost',width:'90px',mono:true,accent:true,render:r=>fmt.usd(r.cost_usd)},
      {key:'s',label:'',width:'50px',render:r=>html`<${Badge} text=${r.status==='ok'||r.status_code===200||!r.status_code?'ok':'err'} variant=${r.status==='ok'||r.status_code===200||!r.status_code?'success':'danger'}/>`}
    ]} rows=${d('traces')} onRowClick=${r=>setDetail(r)} emptyMsg="No traces yet \u2014 send requests to /v1/chat/completions."/>`:
    tab==='costs'?html`<${DataTable} columns=${[{key:'provider',label:'Provider',width:'1fr',mono:true},{key:'model',label:'Model',width:'1fr',mono:true},{key:'reqs',label:'Requests',width:'110px',mono:true,render:r=>fmt.num(r.requests||r.count)},{key:'cost',label:'Cost',width:'110px',mono:true,accent:true,render:r=>fmt.usd(r.cost||r.total_cost)}]} rows=${d('costs')} emptyMsg="No cost data yet."/>`:
    tab==='alerts'?html`<${DataTable} columns=${[{key:'name',label:'Alert',width:'1.5fr'},{key:'metric',label:'Metric',width:'1fr',mono:true},{key:'threshold',label:'Threshold',width:'110px',mono:true},{key:'status',label:'Status',width:'100px',render:r=>html`<${Badge} text=${r.last_fired?'fired':'active'} variant=${r.last_fired?'danger':'success'}/>`}]} rows=${d('alerts')} emptyMsg="No alerts configured."/>`:
    html`<div class="safety-dash">
      <div class="stats-row" style="margin-bottom:16px">
        <${Stat} label="Safety Score" value=${(safety?.safety_score??100)+'/100'} accent=${(safety?.safety_score??100)>=80} sub=${(safety?.safety_score??100)>=80?'healthy':'needs attention'}/>
        <${Stat} label="Events Today" value=${safety?.today?.total||0} sub=${'blocked: '+(safety?.today?.blocked||0)+' \u2022 redacted: '+(safety?.today?.redacted||0)}/>
        <${Stat} label="Active Shields" value=${(safety?.active_modules||0)+(safety?.active_policies||0)} sub=${(safety?.active_modules||0)+' modules \u2022 '+(safety?.active_policies||0)+' policies'}/>
        <${Stat} label="All Time" value=${safety?.total_events||0} sub=${'crit: '+(safety?.severity?.critical||0)+' \u2022 high: '+(safety?.severity?.high||0)+' \u2022 med: '+(safety?.severity?.medium||0)}/>
      </div>
      <div class="section-title" style="margin-bottom:8px">Active Safety Middleware</div>
      <div style="display:flex;gap:8px;flex-wrap:wrap;margin-bottom:16px">
        ${['promptguard','secretscan','toxicfilter','trust_enforce','ipfence','ratelimit','agentguard','codefence','hallucicheck','guardrail','agegate','scopeguard','maskmode'].map(m=>html`<span key=${m} class="badge success" style="padding:3px 8px">${m}</span>`)}
      </div>
      <div class="section-title" style="margin-bottom:8px">Safety Event Log</div>
      <${DataTable} columns=${[
        {key:'ts',label:'Time',width:'100px',render:r=>fmt.ago(r.created_at)},
        {key:'type',label:'Event',width:'140px',mono:true,render:r=>html`<span style="color:${r.severity==='critical'?'var(--red)':r.severity==='high'?'var(--rust-light)':'var(--cream-muted)'}">${r.event_type}</span>`},
        {key:'sev',label:'Severity',width:'90px',render:r=>html`<${Badge} text=${r.severity} variant=${r.severity==='critical'||r.severity==='high'?'danger':r.severity==='medium'?'warn':'muted'}/>`},
        {key:'action',label:'Action',width:'90px',render:r=>html`<${Badge} text=${r.action_taken||'log'} variant=${r.action_taken==='block'?'danger':r.action_taken==='redact'?'warn':'muted'}/>`},
        {key:'cat',label:'Category',width:'110px',mono:true,render:r=>r.category||'\u2014'},
        {key:'model',label:'Model',width:'1fr',mono:true,render:r=>r.model||'\u2014'}
      ]} rows=${safetyEvents} emptyMsg="No safety events \u2014 events appear when safety middlewares detect PII, injections, secrets, or toxic content."/>
    </div>`}
    ${detail&&html`<${Modal} title="Trace Detail" onClose=${()=>setDetail(null)}><div class="trace-detail">
      <div class="td-row"><span class="td-label">Model</span><span class="mono">${detail.model||'\u2014'}</span></div>
      <div class="td-row"><span class="td-label">Provider</span><span class="mono">${detail.provider||'\u2014'}</span></div>
      <div class="td-row"><span class="td-label">Tokens</span><span class="mono">${(detail.tokens_in||0)+' in / '+(detail.tokens_out||0)+' out'}</span></div>
      <div class="td-row"><span class="td-label">Latency</span><span class="mono">${fmt.ms(detail.duration_ms||detail.latency_ms)}</span></div>
      <div class="td-row"><span class="td-label">Cost</span><span class="mono" style="color:var(--gold)">${fmt.usd(detail.cost_usd)}</span></div>
      <div class="td-row"><span class="td-label">Status</span><span class="mono">${detail.status||detail.status_code||200}</span></div>
      <div class="td-row"><span class="td-label">Time</span><span class="mono">${detail.timestamp||detail.created_at||'\u2014'}</span></div>
      ${detail.user_id?html`<div class="td-row"><span class="td-label">User ID</span><span class="mono">#${detail.user_id}</span></div>`:''}
      ${detail.request_body?html`<div class="td-section"><div class="td-label">Request</div><pre class="td-pre">${typeof detail.request_body==='string'?detail.request_body:JSON.stringify(detail.request_body,null,2)}</pre></div>`:''}
      ${detail.response_body?html`<div class="td-section"><div class="td-label">Response</div><pre class="td-pre">${typeof detail.response_body==='string'?detail.response_body:JSON.stringify(detail.response_body,null,2)}</pre></div>`:''}
    </div><//>}`
}

