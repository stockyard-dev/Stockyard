// ─── Growth Dashboard (Internal Mission Control) ────────────────────
function GrowthView(){
  const[data,setData]=useState(null);const[period,setPeriod]=useState('30d');const[loading,setLoading]=useState(true);
  const[toast,setToast]=useState(null);const[showAdd,setShowAdd]=useState(false);const[showRaw,setShowRaw]=useState(false);
  const[addCat,setAddCat]=useState('google_ads');const[addMetric,setAddMetric]=useState('spend');const[addValue,setAddValue]=useState('');const[addDate,setAddDate]=useState(new Date().toISOString().split('T')[0]);const[addNotes,setAddNotes]=useState('');

  const load=async()=>{setLoading(true);const r=await api('/api/growth?period='+period);if(!r._error)setData(r);setLoading(false)};
  useEffect(()=>{load()},[period]);

  const saveMetric=async()=>{const r=await api('/api/growth/metrics',{method:'POST',body:JSON.stringify({date:addDate,category:addCat,metric:addMetric,value:parseFloat(addValue)||0,source:'manual',notes:addNotes})});if(r.status==='saved'){setShowAdd(false);setAddValue('');setAddNotes('');setToast({msg:'Metric saved',type:'success'});load()}else{setToast({msg:'Error saving',type:'error'})}};

  const fmtN=v=>v==null||v===undefined?'\u2014':typeof v==='number'?(v>=1e6?(v/1e6).toFixed(1)+'M':v>=1e3?(v/1e3).toFixed(1)+'K':v%1===0?String(v):v.toFixed(2)):String(v);
  const fmtUSD=v=>v==null?'\u2014':'$'+Number(v).toFixed(2);
  const fmtPct=v=>v==null?'\u2014':Number(v).toFixed(2)+'%';

  const periodLabel={'today':'Today','7d':'Last 7 days','30d':'Last 30 days','90d':'Last 90 days'};
  const srcBadge=s=>html`<span style="font-size:0.5rem;padding:1px 4px;border:1px solid ${s==='auto'?'var(--green)':s==='mixed'?'var(--gold)':'var(--leather)'};color:${s==='auto'?'var(--green-light)':s==='mixed'?'var(--gold)':'var(--leather-light)'};font-family:var(--font-mono);letter-spacing:1px;text-transform:uppercase;vertical-align:middle">${s}</span>`;
  const windowTag=w=>html`<span style="font-size:0.55rem;color:var(--cream-muted);font-family:var(--font-mono);margin-left:6px">${w}</span>`;

  if(loading&&!data)return html`<div class="page-head"><div class="page-eyebrow">Growth</div><h2>Loading...</h2></div>`;

  const d=data||{};
  const inst=d.installs||{};const usr=d.users||{};const prx=d.proxy||{};const nur=d.nurture||{};
  const gads=d.google_ads||{};const rads=d.reddit_ads||{};const gh=d.github||{};const rev=d.revenue||{};

  const spark=(byDay,key)=>{
    if(!byDay||!byDay.length)return null;
    const vals=byDay.map(d=>d[key||'total']||d.count||0);
    const max=Math.max(...vals,1);const w=200;const h=40;
    const points=vals.map((v,i)=>`${(i/(Math.max(vals.length-1,1)))*w},${h-((v/max)*h)}`).join(' ');
    return html`<svg width=${w} height=${h} style="display:block;margin-top:8px"><polyline points=${points} fill="none" stroke="var(--rust-light)" stroke-width="1.5"/></svg>`;
  };

  const pLabel=periodLabel[period]||period;

  return html`<div class="page-head"><div class="page-eyebrow">Mission Control</div><h2>Growth Dashboard</h2><p class="page-sub">Internal traction metrics. Period: <strong>${pLabel}</strong></p></div>
    <div style="display:flex;gap:8px;margin-bottom:20px;font-family:var(--font-mono);font-size:0.72rem;align-items:center">
      ${['today','7d','30d','90d'].map(p=>html`<button key=${p} class="btn btn-sm ${period===p?'primary':''}" onClick=${()=>setPeriod(p)}>${p}</button>`)}
      <div style="margin-left:auto;display:flex;gap:8px">
        <${Btn} small onClick=${()=>setShowRaw(!showRaw)}>${showRaw?'Hide':'Show'} Raw<//>
        <${Btn} small variant="primary" onClick=${()=>setShowAdd(true)}>+ Add Metric<//>
      </div>
    </div>

    <!-- ROW 1: Top-line traction -->
    <div style="display:grid;grid-template-columns:repeat(4,1fr);gap:12px;margin-bottom:24px">
      <div class="stat">
        <div class="stat-label">Total Downloads ${srcBadge('mixed')} ${windowTag('lifetime')}</div>
        <div class="stat-value accent">${fmtN((inst.unique||0)+(gh.release_downloads||0))}</div>
        <div class="stat-sub">${fmtN(inst.unique||0)} site + ${fmtN(gh.release_downloads||0)} GitHub</div>
      </div>
      <div class="stat">
        <div class="stat-label">App Users ${srcBadge('auto')} ${windowTag('lifetime')}</div>
        <div class="stat-value">${fmtN(usr.total)}</div>
        <div class="stat-sub">${fmtN(usr.period)} new in ${pLabel.toLowerCase()}</div>
      </div>
      <div class="stat">
        <div class="stat-label">Proxy Requests ${srcBadge('auto')} ${windowTag(pLabel)}</div>
        <div class="stat-value">${fmtN(prx.period_requests)}</div>
        <div class="stat-sub">${fmtN(prx.total_requests)} lifetime</div>
      </div>
      <div class="stat">
        <div class="stat-label">Teams ${srcBadge('auto')} ${windowTag('lifetime')}</div>
        <div class="stat-value">${fmtN(d.teams?.total)}</div>
        <div class="stat-sub">active namespaces</div>
      </div>
    </div>

    <!-- ROW 2: Installs trend + Email Nurture -->
    <div style="display:grid;grid-template-columns:2fr 1fr;gap:16px;margin-bottom:24px">
      <div style="background:var(--bg2);border:1px solid var(--bg3);padding:16px">
        <div style="font-family:var(--font-mono);font-size:0.68rem;letter-spacing:2px;text-transform:uppercase;color:var(--rust);margin-bottom:8px">Site Installs Over Time ${srcBadge('auto')} ${windowTag(pLabel)}</div>
        ${spark(inst.by_day,'total')}
        ${inst.by_day&&inst.by_day.length>0?html`<div style="display:flex;gap:16px;margin-top:8px;font-size:0.72rem;color:var(--cream-dim);font-family:var(--font-mono)">
          <span>24h: ${fmtN(inst.last_24h?.unique||0)} unique</span><span>7d: ${fmtN(inst.last_7d?.unique||0)}</span><span>30d: ${fmtN(inst.last_30d?.unique||0)}</span>
        </div>`:html`<div style="font-size:0.78rem;color:var(--cream-dim);padding:12px 0">No install data yet</div>`}
      </div>
      <div style="background:var(--bg2);border:1px solid var(--bg3);padding:16px">
        <div style="font-family:var(--font-mono);font-size:0.68rem;letter-spacing:2px;text-transform:uppercase;color:var(--rust);margin-bottom:12px">Email Nurture ${srcBadge('auto')} ${windowTag('lifetime')}</div>
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
        <div style="font-family:var(--font-mono);font-size:0.68rem;letter-spacing:2px;text-transform:uppercase;color:var(--rust);margin-bottom:12px">Google Ads ${srcBadge('manual')} ${windowTag(pLabel)}</div>
        ${gads.total_spend>0?html`
          <div style="display:grid;grid-template-columns:1fr 1fr 1fr;gap:8px;font-size:0.78rem;color:var(--cream-dim)">
            <div><div style="color:var(--leather);font-size:0.55rem;text-transform:uppercase;margin-bottom:2px">Spend</div><div style="color:var(--cream)">${fmtUSD(gads.total_spend)}</div></div>
            <div><div style="color:var(--leather);font-size:0.55rem;text-transform:uppercase;margin-bottom:2px">Clicks</div><div style="color:var(--cream)">${fmtN(gads.total_clicks)}</div></div>
            <div><div style="color:var(--leather);font-size:0.55rem;text-transform:uppercase;margin-bottom:2px">CTR</div><div style="color:var(--cream)">${fmtPct(gads.ctr)}</div></div>
            <div><div style="color:var(--leather);font-size:0.55rem;text-transform:uppercase;margin-bottom:2px">CPC</div><div style="color:var(--cream)">${fmtUSD(gads.cpc)}</div></div>
            <div><div style="color:var(--leather);font-size:0.55rem;text-transform:uppercase;margin-bottom:2px">Impressions</div><div style="color:var(--cream)">${fmtN(gads.total_impressions)}</div></div>
          </div>
        `:html`<div style="font-size:0.78rem;color:var(--cream-muted);padding:8px 0">No data. Click + Add Metric.</div>`}
      </div>
      <div style="background:var(--bg2);border:1px solid var(--bg3);padding:16px">
        <div style="font-family:var(--font-mono);font-size:0.68rem;letter-spacing:2px;text-transform:uppercase;color:var(--rust);margin-bottom:12px">Reddit Ads ${srcBadge('manual')} ${windowTag(pLabel)}</div>
        ${rads.total_spend>0?html`
          <div style="display:grid;grid-template-columns:1fr 1fr 1fr;gap:8px;font-size:0.78rem;color:var(--cream-dim)">
            <div><div style="color:var(--leather);font-size:0.55rem;text-transform:uppercase;margin-bottom:2px">Spend</div><div style="color:var(--cream)">${fmtUSD(rads.total_spend)}</div></div>
            <div><div style="color:var(--leather);font-size:0.55rem;text-transform:uppercase;margin-bottom:2px">Clicks</div><div style="color:var(--cream)">${fmtN(rads.total_clicks)}</div></div>
            <div><div style="color:var(--leather);font-size:0.55rem;text-transform:uppercase;margin-bottom:2px">CTR</div><div style="color:var(--cream)">${fmtPct(rads.ctr)}</div></div>
            <div><div style="color:var(--leather);font-size:0.55rem;text-transform:uppercase;margin-bottom:2px">CPC</div><div style="color:var(--cream)">${fmtUSD(rads.cpc)}</div></div>
            <div><div style="color:var(--leather);font-size:0.55rem;text-transform:uppercase;margin-bottom:2px">Impressions</div><div style="color:var(--cream)">${fmtN(rads.total_impressions)}</div></div>
          </div>
        `:html`<div style="font-size:0.78rem;color:var(--cream-muted);padding:8px 0">No data. Click + Add Metric.</div>`}
      </div>
    </div>

    <!-- ROW 4: GitHub (trust signals) + Revenue -->
    <div style="display:grid;grid-template-columns:1fr 1fr;gap:16px;margin-bottom:24px">
      <div style="background:var(--bg2);border:1px solid var(--bg3);padding:16px">
        <div style="font-family:var(--font-mono);font-size:0.68rem;letter-spacing:2px;text-transform:uppercase;color:var(--rust);margin-bottom:12px">GitHub ${srcBadge('manual')} ${windowTag('snapshot')}</div>
        <div style="display:grid;grid-template-columns:1fr 1fr 1fr 1fr;gap:8px;font-size:0.78rem;color:var(--cream-dim)">
          <div><div style="color:var(--leather);font-size:0.55rem;text-transform:uppercase;margin-bottom:2px">Stars</div><div style="color:var(--cream)">${fmtN(gh.stars)||'\u2014'}</div></div>
          <div><div style="color:var(--leather);font-size:0.55rem;text-transform:uppercase;margin-bottom:2px">Visitors (14d)</div><div style="color:var(--cream)">${fmtN(gh.visitors)||'\u2014'}</div></div>
          <div><div style="color:var(--leather);font-size:0.55rem;text-transform:uppercase;margin-bottom:2px">Views (14d)</div><div style="color:var(--cream)">${fmtN(gh.views_14d)||'\u2014'}</div></div>
          <div><div style="color:var(--leather);font-size:0.55rem;text-transform:uppercase;margin-bottom:2px">Release DLs</div><div style="color:var(--cream)">${fmtN(gh.release_downloads)||'\u2014'}</div></div>
        </div>
        <div style="font-size:0.6rem;color:var(--cream-muted);margin-top:8px;font-style:italic">Stars + unique visitors are trustworthy signals. Clones excluded (noisy).</div>
      </div>
      <div style="background:var(--bg2);border:1px solid var(--bg3);padding:16px">
        <div style="font-family:var(--font-mono);font-size:0.68rem;letter-spacing:2px;text-transform:uppercase;color:var(--rust);margin-bottom:12px">Revenue ${srcBadge('mixed')} ${windowTag(pLabel)}</div>
        <div style="display:grid;grid-template-columns:1fr 1fr 1fr;gap:8px;font-size:0.78rem;color:var(--cream-dim)">
          <div><div style="color:var(--leather);font-size:0.55rem;text-transform:uppercase;margin-bottom:2px">MRR</div><div style="color:var(--cream)">${rev.mrr!=null?fmtUSD(rev.mrr):'\u2014'}</div></div>
          <div><div style="color:var(--leather);font-size:0.55rem;text-transform:uppercase;margin-bottom:2px">Total Revenue</div><div style="color:var(--cream)">${rev.total_revenue!=null?fmtUSD(rev.total_revenue):'\u2014'}</div></div>
          <div><div style="color:var(--leather);font-size:0.55rem;text-transform:uppercase;margin-bottom:2px">New Customers</div><div style="color:var(--cream)">${rev.new_customers!=null?fmtN(rev.new_customers):'\u2014'}</div></div>
        </div>
        ${rev.users_by_tier&&Object.keys(rev.users_by_tier).length>0?html`
          <div style="margin-top:12px;padding-top:8px;border-top:1px solid var(--bg3)">
            <div style="color:var(--leather);font-size:0.55rem;text-transform:uppercase;margin-bottom:6px">Users by Tier ${srcBadge('auto')}</div>
            <div style="display:flex;gap:12px;font-size:0.72rem;font-family:var(--font-mono)">
              ${Object.entries(rev.users_by_tier||{}).map(([t,c])=>html`<span style="color:var(--cream-dim)">${t}: <span style="color:var(--cream)">${c}</span></span>`)}
            </div>
          </div>
        `:null}
      </div>
    </div>

    <!-- Signups over time -->
    ${usr.by_day&&usr.by_day.length>0?html`
    <div style="background:var(--bg2);border:1px solid var(--bg3);padding:16px;margin-bottom:24px">
      <div style="font-family:var(--font-mono);font-size:0.68rem;letter-spacing:2px;text-transform:uppercase;color:var(--rust);margin-bottom:8px">App Users Over Time ${srcBadge('auto')} ${windowTag(pLabel)}</div>
      ${spark(usr.by_day,'count')}
    </div>`:null}

    <!-- Install Methods -->
    ${inst.top_agents&&inst.top_agents.length>0?html`
    <div style="background:var(--bg2);border:1px solid var(--bg3);padding:16px;margin-bottom:24px">
      <div style="font-family:var(--font-mono);font-size:0.68rem;letter-spacing:2px;text-transform:uppercase;color:var(--rust);margin-bottom:8px">Install Methods ${srcBadge('auto')} ${windowTag('lifetime')}</div>
      <div style="display:grid;gap:4px;font-size:0.75rem;font-family:var(--font-mono)">
        ${inst.top_agents.map(a=>html`<div style="display:flex;justify-content:space-between;color:var(--cream-dim)">
          <span style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap;max-width:80%">${a.user_agent||'(empty)'}</span>
          <span style="color:var(--cream)">${a.count}</span>
        </div>`)}
      </div>
    </div>`:null}

    <!-- RAW METRICS (toggled, secondary) -->
    ${showRaw?html`
    <div style="background:var(--bg2);border:1px solid var(--bg3);padding:16px;margin-bottom:24px">
      <div style="font-family:var(--font-mono);font-size:0.68rem;letter-spacing:2px;text-transform:uppercase;color:var(--leather);margin-bottom:12px">Raw / Source Metrics</div>
      <div style="display:grid;grid-template-columns:repeat(3,1fr);gap:12px;font-size:0.75rem;font-family:var(--font-mono);color:var(--cream-dim)">
        <div>
          <div style="color:var(--leather);font-size:0.55rem;text-transform:uppercase;margin-bottom:4px">GitHub Clones (14d) ${srcBadge('manual')}</div>
          <div style="color:var(--cream)">${fmtN(gh.clones)||'\u2014'}</div>
          <div style="font-size:0.6rem;color:var(--cream-muted);font-style:italic;margin-top:2px">Inflated by bots, CI, mirrors</div>
        </div>
        <div>
          <div style="color:var(--leather);font-size:0.55rem;text-transform:uppercase;margin-bottom:4px">Total Proxy Cost ${srcBadge('auto')}</div>
          <div style="color:var(--cream)">${fmtUSD(prx.total_cost_usd)}</div>
          <div style="font-size:0.6rem;color:var(--cream-muted);font-style:italic;margin-top:2px">${fmtUSD(prx.period_cost_usd)} in ${pLabel.toLowerCase()}</div>
        </div>
        <div>
          <div style="color:var(--leather);font-size:0.55rem;text-transform:uppercase;margin-bottom:4px">Install Requests (total) ${srcBadge('auto')}</div>
          <div style="color:var(--cream)">${fmtN(inst.total)}</div>
          <div style="font-size:0.6rem;color:var(--cream-muted);font-style:italic;margin-top:2px">Includes repeat/automated hits</div>
        </div>
      </div>
    </div>`:null}

    <div style="font-family:var(--font-mono);font-size:0.6rem;color:var(--leather);text-align:center;margin-top:2rem;padding-top:1rem;border-top:1px solid var(--bg3)">
      Generated ${d.generated?new Date(d.generated).toLocaleString():'\u2014'} \u00b7 ${pLabel} \u00b7 <span style="color:var(--green-light)">\u25a0</span> auto <span style="color:var(--leather-light)">\u25a0</span> manual <span style="color:var(--gold)">\u25a0</span> mixed
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
          [{value:'stars',label:'Stars'},{value:'visitors',label:'Unique Visitors (14d)'},{value:'release_downloads',label:'Release Downloads'},{value:'clones',label:'Clones (14d, raw)'}]:
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
