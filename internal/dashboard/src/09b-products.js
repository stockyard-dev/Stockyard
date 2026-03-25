
// ── Products view: platform product grid with tier gating ──
function ProductsView(){
  const[products,setProducts]=useState(null);const[health,setHealth]=useState(null);const[events,setEvents]=useState([]);const[tab,setTab]=useState('all');const[toast,setToast]=useState(null);
  const load=async()=>{const p=await api('/api/platform/products');if(!p._error)setProducts(p);const h=await api('/api/platform/health');if(!h._error)setHealth(h)};
  useEffect(()=>{load();const i=setInterval(load,15000);return()=>clearInterval(i)},[]);
  useEffect(()=>{let es;try{es=new EventSource('/api/platform/events');es.onmessage=e=>{try{const evt=JSON.parse(e.data);setEvents(prev=>[evt,...prev].slice(0,50))}catch(ex){}};es.onerror=()=>{}}catch(ex){}return()=>{if(es)es.close()}},[]);
  const toggle=async(id,enabled)=>{const r=await api('/api/platform/products/'+id+'/toggle',{method:'PUT',body:JSON.stringify({enabled})});if(!r._error){setToast({type:'success',msg:id+' '+(enabled?'enabled':'disabled')});load()}else{setToast({type:'error',msg:'Failed to toggle '+id})}};
  useEffect(()=>{if(toast){const t=setTimeout(()=>setToast(null),3000);return()=>clearTimeout(t)}},[toast]);
  if(!products)return html`<div class="loading">Loading products\u2026</div>`;
  const all=products.products||[];const tiers=['Individual','Pro','Team','Enterprise'];
  const filtered=tab==='all'?all:tab==='active'?all.filter(p=>p.active):tab==='locked'?all.filter(p=>!p.active):all.filter(p=>p.required_tier_name===tab);
  const tierColor=t=>t==='Individual'?'var(--green)':t==='Pro'?'var(--gold)':t==='Team'?'var(--rust-light)':t==='Enterprise'?'var(--leather-light)':'var(--cream-muted)';
  const catIcon=c=>c==='lifecycle'?'\u25C6':c==='runtime'?'\u25C8':c==='quality'?'\u2B21':c==='infra'?'\u25CE':c==='insight'?'\u25C7':'\u25A3';
  return html`<div>
    <div class="page-head"><div class="page-eyebrow">Platform</div><h2>Products</h2>
      <div class="page-sub">18 products \u2022 ${products.active} active \u2022 ${products.locked} locked \u2022 tier: ${products.current_tier}</div></div>
    ${health&&html`<div class="stats-row">
      <div class="stat"><div class="stat-label">Status</div><div class="stat-value" style="color:${health.status==='ok'?'var(--green)':health.status==='degraded'?'var(--gold)':'var(--red)'}">${health.status}</div></div>
      <div class="stat"><div class="stat-label">Active</div><div class="stat-value accent">${health.active}</div><div class="stat-sub">of ${health.products} products</div></div>
      <div class="stat"><div class="stat-label">Healthy</div><div class="stat-value" style="color:var(--green)">${health.healthy}</div></div>
      <div class="stat"><div class="stat-label">Locked</div><div class="stat-value" style="color:var(--cream-muted)">${health.locked}</div><div class="stat-sub">upgrade to unlock</div></div>
    </div>`}
    <div class="tab-bar">
      ${['all','active','locked',...tiers].map(t=>html`<button key=${t} class="tab ${tab===t?'active':''}" onClick=${()=>setTab(t)}>${t}</button>`)}
    </div>
    <div class="product-grid">
      ${filtered.map(p=>html`<div key=${p.id} class="product-card ${p.active?'':'product-locked'}">
        <div class="product-card-head">
          <span class="product-card-icon" style="color:${p.active?tierColor(p.required_tier_name):'var(--cream-muted)'}">${catIcon(p.category)}</span>
          <span class="product-card-name">${p.name}</span>
          <span class="badge ${p.active?(p.status==='healthy'?'success':p.status==='degraded'?'warn':'danger'):'muted'}">${p.active?p.status:'locked'}</span>
          ${p.active&&html`<button class="toggle-btn ${p.enabled?'on':''}" onClick=${()=>toggle(p.id,!p.enabled)} style="margin-left:auto" title="${p.enabled?'Disable':'Enable'} ${p.name}"><span class="toggle-knob"></span></button>`}
        </div>
        <div class="product-card-desc">${p.description}</div>
        <div class="product-card-meta">
          <span class="product-tier-badge" style="color:${tierColor(p.required_tier_name)};border-color:${tierColor(p.required_tier_name)}">${p.required_tier_name}</span>
          <span class="product-category">${p.category}</span>
          ${p.active?html`<code class="product-api">${p.api_url}</code>`:html`<a class="product-upgrade" href="https://stockyard.dev/pricing" target="_blank">Upgrade \u2192</a>`}
        </div>
      </div>`)}
      ${filtered.length===0&&html`<div class="empty-state">No products match this filter.</div>`}
    </div>
    ${events.length>0&&html`<div style="margin-top:24px">
      <div style="font-family:var(--font-mono);font-size:0.65rem;letter-spacing:2px;text-transform:uppercase;color:var(--leather);margin-bottom:8px;display:flex;align-items:center;gap:8px">
        <span style="width:6px;height:6px;border-radius:50%;background:var(--green);animation:pulse 2s infinite"></span> Event Stream
      </div>
      <div class="live-feed" style="max-height:200px">${events.slice(0,20).map((e,i)=>html`<div key=${e.id||i} class="live-event"><div class="live-event-head">
        <span class="badge" style="font-size:0.6rem">${e.source}</span>
        <span class="mono" style="font-size:0.75rem;color:var(--cream)">${e.type}</span>
        <span class="mono" style="font-size:0.68rem;color:var(--cream-muted);margin-left:auto">${fmt.ago(e.timestamp)}</span>
      </div></div>`)}</div>
    </div>`}
    ${toast&&html`<div class="toast toast-${toast.type}">${toast.msg}</div>`}
  </div>`;
}
