// ─── Growth Dashboard (Internal Mission Control) ────────────────────
function GrowthView(){
  const[data,setData]=useState(null);const[period,setPeriod]=useState('30d');const[loading,setLoading]=useState(true);
  const[toast,setToast]=useState(null);const[showAdd,setShowAdd]=useState(false);
  const[addCat,setAddCat]=useState('google_ads');const[addMetric,setAddMetric]=useState('spend');const[addValue,setAddValue]=useState('');const[addDate,setAddDate]=useState(new Date().toISOString().split('T')[0]);const[addNotes,setAddNotes]=useState('');

  const load=async()=>{setLoading(true);const r=await api('/api/growth?period='+period);if(!r._error)setData(r);setLoading(false)};
  useEffect(()=>{load()},[period]);

  const saveMetric=async()=>{const r=await api('/api/growth/metrics',{method:'POST',body:JSON.stringify({date:addDate,category:addCat,metric:addMetric,value:parseFloat(addValue)||0,source:'manual',notes:addNotes})});if(r.status==='saved'){setShowAdd(false);setAddValue('');setAddNotes('');setToast({msg:'Metric saved',type:'success'});load()}else{setToast({msg:'Error saving',type:'error'})}};

  const fmtN=v=>v==null||v===undefined?'—':typeof v==='number'?(v>=1e6?(v/1e6).toFixed(1)+'M':v>=1e3?(v/1e3).toFixed(1)+'K':v%1===0?String(v):v.toFixed(2)):String(v);
  const fmtUSD=v=>v==null?'—':'$'+Number(v).toFixed(2);
  const fmtPct=v=>v==null?'—':Number(v).toFixed(2)+'%';
  const srcBadge=s=>html`<span style="font-size:0.55rem;padding:1px 5px;border:1px solid ${s==='auto'?'var(--green)':'var(--leather)'};color:${s==='auto'?'var(--green-light)':'var(--leather-light)'};font-family:var(--font-mono);letter-spacing:1px;text-transform:uppercase;margin-left:6px;vertical-align:middle">${s}</span>`;

  if(loading&&!data)return html`<div class="page-head"><div class="page-eyebrow">Growth</div><h2>Loading...</h2></div>`;

  const d=data||{};
  const inst=d.installs||{};const usr=d.users||{};const prx=d.proxy||{};const nur=d.nurture||{};
  const gads=d.google_ads||{};const rads=d.reddit_ads||{};const gh=d.github||{};const rev=d.revenue||{};

  // Simple sparkline from by_day data
  const spark=(byDay,key)=>{
    if(!byDay||!byDay.length)return null;
    const vals=byDay.map(d=>d[key||'total']||d.count||0);
    const max=Math.max(...vals,1);const w=200;const h=40;
    const points=vals.map((v,i)=>`${(i/(vals.length-1))*w},${h-((v/max)*h)}`).join(' ');
    return html`<svg width=${w} height=${h} style="display:block;margin-top:8px"><polyline points=${points} fill="none" stroke="var(--rust-light)" stroke-width="1.5"/></svg>`;
  };

  return html`<div class="page-head"><div class="page-eyebrow">Mission Control</div><h2>Growth Dashboard</h2><p class="page-sub">Internal traction metrics. Auto-tracked + manual entry.</p></div>
    <div style="display:flex;gap:8px;margin-bottom:20px;font-family:var(--font-mono);font-size:0.72rem">
      ${['today','7d','30d','90d'].map(p=>html`<button key=${p} class="btn btn-sm ${period===p?'primary':''}" onClick=${()=>setPeriod(p)}>${p}</button>`)}
      <div style="margin-left:auto"><${Btn} small variant="primary" onClick=${()=>setShowAdd(true)}>+ Add Metric<//></div>
    </div>

    <!-- ROW 1: Core auto-tracked -->
    <div style="display:grid;grid-template-columns:repeat(4,1fr);gap:12px;margin-bottom:24px">
      <div class="stat"><div class="stat-label">Installs (unique) ${srcBadge('auto')}</div><div class="stat-value accent">${fmtN(inst.unique)}</div><div class="stat-sub">${fmtN(inst.total)} total requests</div></div>
      <div class="stat"><div class="stat-label">Users ${srcBadge('auto')}</div><div class="stat-value">${fmtN(usr.total)}</div><div class="stat-sub">${fmtN(usr.period)} in period</div></div>
      <div class="stat"><div class="stat-label">Proxy Requests ${srcBadge('auto')}</div><div class="stat-value">${fmtN(prx.total_requests)}</div><div class="stat-sub">${fmtN(prx.period_requests)} in period</div></div>
      <div class="stat"><div class="stat-label">Teams ${srcBadge('auto')}</div><div class="stat-value">${fmtN(d.teams?.total)}</div><div class="stat-sub">active team namespaces</div></div>
    </div>

    <!-- ROW 2: Installs trend + Nurture -->
    <div style="display:grid;grid-template-columns:2fr 1fr;gap:16px;margin-bottom:24px">
      <div style="background:var(--bg2);border:1px solid var(--bg3);padding:16px">
        <div style="font-family:var(--font-mono);font-size:0.68rem;letter-spacing:2px;text-transform:uppercase;color:var(--rust);margin-bottom:8px">Installs Over Time ${srcBadge('auto')}</div>
        ${spark(inst.by_day,'total')}
        ${inst.by_day&&inst.by_day.length>0?html`<div style="display:flex;gap:16px;margin-top:8px;font-size:0.72rem;color:var(--cream-dim);font-family:var(--font-mono)">
          <span>24h: ${fmtN(inst.last_24h?.total)}</span><span>7d: ${fmtN(inst.last_7d?.total)}</span><span>30d: ${fmtN(inst.last_30d?.total)}</span>
        </div>`:html`<div style="font-size:0.78rem;color:var(--cream-dim);padding:12px 0">No install data yet</div>`}
      </div>
      <div style="background:var(--bg2);border:1px solid var(--bg3);padding:16px">
        <div style="font-family:var(--font-mono);font-size:0.68rem;letter-spacing:2px;text-transform:uppercase;color:var(--rust);margin-bottom:12px">Nurture ${srcBadge('auto')}</div>
        <div style="font-size:0.78rem;color:var(--cream-dim);display:grid;gap:6px">
          <div style="display:flex;justify-content:space-between"><span>Leads captured</span><span style="color:var(--cream)">${fmtN(nur.leads)}</span></div>
          <div style="display:flex;justify-content:space-between"><span>Emails sent</span><span style="color:var(--cream)">${fmtN(nur.emails_sent)}</span></div>
          <div style="display:flex;justify-content:space-between"><span>Failed</span><span style="color:${nur.emails_failed>0?'var(--rust)':'var(--cream)'}">${fmtN(nur.emails_failed)}</span></div>
        </div>
      </div>
    </div>

    <!-- ROW 3: Ads -->
    <div style="display:grid;grid-template-columns:1fr 1fr;gap:16px;margin-bottom:24px">
      <div style="background:var(--bg2);border:1px solid var(--bg3);padding:16px">
        <div style="font-family:var(--font-mono);font-size:0.68rem;letter-spacing:2px;text-transform:uppercase;color:var(--rust);margin-bottom:12px">Google Ads ${srcBadge('manual')}</div>
        ${gads.total_spend>0?html`
          <div style="display:grid;grid-template-columns:1fr 1fr;gap:8px;font-size:0.78rem;color:var(--cream-dim)">
            <div><div style="color:var(--leather);font-size:0.6rem;text-transform:uppercase;margin-bottom:2px">Spend</div><div style="color:var(--cream)">${fmtUSD(gads.total_spend)}</div></div>
            <div><div style="color:var(--leather);font-size:0.6rem;text-transform:uppercase;margin-bottom:2px">Clicks</div><div style="color:var(--cream)">${fmtN(gads.total_clicks)}</div></div>
            <div><div style="color:var(--leather);font-size:0.6rem;text-transform:uppercase;margin-bottom:2px">Impressions</div><div style="color:var(--cream)">${fmtN(gads.total_impressions)}</div></div>
            <div><div style="color:var(--leather);font-size:0.6rem;text-transform:uppercase;margin-bottom:2px">CTR</div><div style="color:var(--cream)">${fmtPct(gads.ctr)}</div></div>
            <div><div style="color:var(--leather);font-size:0.6rem;text-transform:uppercase;margin-bottom:2px">CPC</div><div style="color:var(--cream)">${fmtUSD(gads.cpc)}</div></div>
          </div>
        `:html`<div style="font-size:0.78rem;color:var(--cream-muted);padding:8px 0">No data yet. Click + Add Metric to enter Google Ads stats.</div>`}
      </div>
      <div style="background:var(--bg2);border:1px solid var(--bg3);padding:16px">
        <div style="font-family:var(--font-mono);font-size:0.68rem;letter-spacing:2px;text-transform:uppercase;color:var(--rust);margin-bottom:12px">Reddit Ads ${srcBadge('manual')}</div>
        ${rads.total_spend>0?html`
          <div style="display:grid;grid-template-columns:1fr 1fr;gap:8px;font-size:0.78rem;color:var(--cream-dim)">
            <div><div style="color:var(--leather);font-size:0.6rem;text-transform:uppercase;margin-bottom:2px">Spend</div><div style="color:var(--cream)">${fmtUSD(rads.total_spend)}</div></div>
            <div><div style="color:var(--leather);font-size:0.6rem;text-transform:uppercase;margin-bottom:2px">Clicks</div><div style="color:var(--cream)">${fmtN(rads.total_clicks)}</div></div>
            <div><div style="color:var(--leather);font-size:0.6rem;text-transform:uppercase;margin-bottom:2px">Impressions</div><div style="color:var(--cream)">${fmtN(rads.total_impressions)}</div></div>
            <div><div style="color:var(--leather);font-size:0.6rem;text-transform:uppercase;margin-bottom:2px">CTR</div><div style="color:var(--cream)">${fmtPct(rads.ctr)}</div></div>
            <div><div style="color:var(--leather);font-size:0.6rem;text-transform:uppercase;margin-bottom:2px">CPC</div><div style="color:var(--cream)">${fmtUSD(rads.cpc)}</div></div>
          </div>
        `:html`<div style="font-size:0.78rem;color:var(--cream-muted);padding:8px 0">No data yet. Click + Add Metric to enter Reddit Ads stats.</div>`}
      </div>
    </div>

    <!-- ROW 4: GitHub + Revenue -->
    <div style="display:grid;grid-template-columns:1fr 1fr;gap:16px;margin-bottom:24px">
      <div style="background:var(--bg2);border:1px solid var(--bg3);padding:16px">
        <div style="font-family:var(--font-mono);font-size:0.68rem;letter-spacing:2px;text-transform:uppercase;color:var(--rust);margin-bottom:12px">GitHub ${srcBadge('manual')}</div>
        <div style="display:grid;grid-template-columns:1fr 1fr;gap:8px;font-size:0.78rem;color:var(--cream-dim)">
          <div><div style="color:var(--leather);font-size:0.6rem;text-transform:uppercase;margin-bottom:2px">Stars</div><div style="color:var(--cream)">${fmtN(gh.stars)||'—'}</div></div>
          <div><div style="color:var(--leather);font-size:0.6rem;text-transform:uppercase;margin-bottom:2px">Clones</div><div style="color:var(--cream)">${fmtN(gh.clones)||'—'}</div></div>
          <div><div style="color:var(--leather);font-size:0.6rem;text-transform:uppercase;margin-bottom:2px">Visitors</div><div style="color:var(--cream)">${fmtN(gh.visitors)||'—'}</div></div>
          <div><div style="color:var(--leather);font-size:0.6rem;text-transform:uppercase;margin-bottom:2px">Release DLs</div><div style="color:var(--cream)">${fmtN(gh.release_downloads)||'—'}</div></div>
        </div>
      </div>
      <div style="background:var(--bg2);border:1px solid var(--bg3);padding:16px">
        <div style="font-family:var(--font-mono);font-size:0.68rem;letter-spacing:2px;text-transform:uppercase;color:var(--rust);margin-bottom:12px">Revenue ${srcBadge('mixed')}</div>
        <div style="display:grid;grid-template-columns:1fr 1fr;gap:8px;font-size:0.78rem;color:var(--cream-dim)">
          <div><div style="color:var(--leather);font-size:0.6rem;text-transform:uppercase;margin-bottom:2px">MRR</div><div style="color:var(--cream)">${rev.mrr!=null?fmtUSD(rev.mrr):'—'}</div></div>
          <div><div style="color:var(--leather);font-size:0.6rem;text-transform:uppercase;margin-bottom:2px">Total Revenue</div><div style="color:var(--cream)">${rev.total_revenue!=null?fmtUSD(rev.total_revenue):'—'}</div></div>
          <div><div style="color:var(--leather);font-size:0.6rem;text-transform:uppercase;margin-bottom:2px">New Customers</div><div style="color:var(--cream)">${rev.new_customers!=null?fmtN(rev.new_customers):'—'}</div></div>
        </div>
        ${rev.users_by_tier&&Object.keys(rev.users_by_tier).length>0?html`
          <div style="margin-top:12px;padding-top:8px;border-top:1px solid var(--bg3)">
            <div style="color:var(--leather);font-size:0.6rem;text-transform:uppercase;margin-bottom:6px">Users by Tier ${srcBadge('auto')}</div>
            <div style="display:flex;gap:12px;font-size:0.72rem;font-family:var(--font-mono)">
              ${Object.entries(rev.users_by_tier||{}).map(([t,c])=>html`<span style="color:var(--cream-dim)">${t}: <span style="color:var(--cream)">${c}</span></span>`)}
            </div>
          </div>
        `:null}
      </div>
    </div>

    <!-- ROW 5: Users over time -->
    ${usr.by_day&&usr.by_day.length>0?html`
    <div style="background:var(--bg2);border:1px solid var(--bg3);padding:16px;margin-bottom:24px">
      <div style="font-family:var(--font-mono);font-size:0.68rem;letter-spacing:2px;text-transform:uppercase;color:var(--rust);margin-bottom:8px">Signups Over Time ${srcBadge('auto')}</div>
      ${spark(usr.by_day,'count')}
    </div>`:null}

    <!-- ROW 6: Top Install Agents -->
    ${inst.top_agents&&inst.top_agents.length>0?html`
    <div style="background:var(--bg2);border:1px solid var(--bg3);padding:16px;margin-bottom:24px">
      <div style="font-family:var(--font-mono);font-size:0.68rem;letter-spacing:2px;text-transform:uppercase;color:var(--rust);margin-bottom:8px">Install Methods ${srcBadge('auto')}</div>
      <div style="display:grid;gap:4px;font-size:0.75rem;font-family:var(--font-mono)">
        ${inst.top_agents.map(a=>html`<div style="display:flex;justify-content:space-between;color:var(--cream-dim)">
          <span style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap;max-width:80%">${a.user_agent||'(empty)'}</span>
          <span style="color:var(--cream)">${a.count}</span>
        </div>`)}
      </div>
    </div>`:null}

    <!-- Proxy cost -->
    <div style="display:grid;grid-template-columns:1fr 1fr;gap:16px;margin-bottom:24px">
      <div class="stat"><div class="stat-label">Total Proxy Cost ${srcBadge('auto')}</div><div class="stat-value">${fmtUSD(prx.total_cost_usd)}</div><div class="stat-sub">${fmtUSD(prx.period_cost_usd)} in period</div></div>
      <div class="stat"><div class="stat-label">Proxy Cost (period) ${srcBadge('auto')}</div><div class="stat-value">${fmtN(prx.period_requests)}</div><div class="stat-sub">requests in period</div></div>
    </div>

    <div style="font-family:var(--font-mono);font-size:0.6rem;color:var(--leather);text-align:center;margin-top:2rem;padding-top:1rem;border-top:1px solid var(--bg3)">
      Generated ${d.generated?new Date(d.generated).toLocaleString():'—'} · Period: ${period} · <span style="color:var(--green-light)">■</span> auto <span style="color:var(--leather-light)">■</span> manual <span style="color:var(--cream-muted)">■</span> mixed
    </div>

    ${showAdd&&html`<${Modal} title="Add Metric" onClose=${()=>setShowAdd(false)}>
      <${Select} label="Category" value=${addCat} onChange=${setAddCat} options=${[
        {value:'google_ads',label:'Google Ads'},{value:'reddit_ads',label:'Reddit Ads'},
        {value:'github',label:'GitHub'},{value:'revenue',label:'Revenue'}
      ]}/>
      <${Select} label="Metric" value=${addMetric} onChange=${setAddMetric} options=${
        addCat==='google_ads'||addCat==='reddit_ads'?
          [{value:'spend',label:'Spend ($)'},{value:'impressions',label:'Impressions'},{value:'clicks',label:'Clicks'}]:
        addCat==='github'?
          [{value:'stars',label:'Stars'},{value:'clones',label:'Clones (14d)'},{value:'visitors',label:'Visitors (14d)'},{value:'release_downloads',label:'Release Downloads'}]:
          [{value:'mrr',label:'MRR ($)'},{value:'total_revenue',label:'Total Revenue ($)'},{value:'new_customers',label:'New Customers'}]
      }/>
      <${Input} label="Date" value=${addDate} onChange=${setAddDate} placeholder="2026-03-29"/>
      <${Input} label="Value" value=${addValue} onChange=${setAddValue} placeholder="0" mono/>
      <${Input} label="Notes (optional)" value=${addNotes} onChange=${setAddNotes} placeholder="e.g. weekly snapshot"/>
      <div style="margin-top:16px;display:flex;gap:8px;justify-content:flex-end">
        <${Btn} onClick=${()=>setShowAdd(false)}>Cancel<//>
        <${Btn} variant="primary" onClick=${saveMetric} disabled=${!addValue}>Save Metric<//>
      </div>
    <//>`}
    ${toast&&html`<${Toast} msg=${toast.msg} type=${toast.type} onDone=${()=>setToast(null)}/>`}`;
}
