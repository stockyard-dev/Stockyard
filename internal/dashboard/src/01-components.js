// ─── Shared Components ─────────────────────────────────────────────
function Stat({label,value,sub,accent}){return html`<div class="stat"><div class="stat-label">${label}</div><div class="stat-value ${accent?'accent':''}">${value}</div>${sub&&html`<div class="stat-sub">${sub}</div>`}</div>`}
function Badge({text,variant}){return html`<span class="badge ${variant||''}">${text}</span>`}
function TabBar({tabs,active,onChange}){return html`<div class="tab-bar">${tabs.map(t=>html`<button key=${t} class="tab ${active===t?'active':''}" onClick=${()=>onChange(t)}>${t}</button>`)}</div>`}
function Toast({msg,type,onDone}){useEffect(()=>{const t=setTimeout(onDone,3000);return()=>clearTimeout(t)},[]);return html`<div class="toast toast-${type||'info'}">${msg}</div>`}
function Btn({children,onClick,variant,disabled,small}){return html`<button class="btn ${variant||''} ${small?'btn-sm':''}" onClick=${onClick} disabled=${disabled}>${children}</button>`}
function Input({label,value,onChange,type,placeholder,mono}){return html`<div class="field">${label&&html`<label class="field-label">${label}</label>`}<input type=${type||'text'} value=${value} onInput=${e=>onChange(e.target.value)} class="field-input ${mono?'mono':''}" placeholder=${placeholder||''}/></div>`}
function Select({label,value,onChange,options}){return html`<div class="field">${label&&html`<label class="field-label">${label}</label>`}<select class="field-input" value=${value} onChange=${e=>onChange(e.target.value)}>${options.map(o=>html`<option key=${o.value||o} value=${o.value||o}>${o.label||o}</option>`)}</select></div>`}
function Modal({title,onClose,children}){return html`<div class="modal-overlay" onClick=${e=>{if(e.target===e.currentTarget)onClose()}}><div class="modal"><div class="modal-head"><span class="modal-title">${title}</span><button class="modal-close" onClick=${onClose}>\u2715</button></div><div class="modal-body">${children}</div></div></div>`}
function DataTable({columns,rows,emptyMsg,onRowClick}){
  if(!rows||rows.length===0)return html`<div class="empty-state">${emptyMsg||'No data yet.'}</div>`;
  const gc=columns.map(c=>c.width||'1fr').join(' ');
  return html`<div class="data-table"><div class="dt-head" style="grid-template-columns:${gc}">${columns.map(c=>html`<span key=${c.key}>${c.label}</span>`)}</div><div class="dt-body">${rows.map((row,i)=>html`<div key=${row.id||i} class="dt-row ${onRowClick?'dt-clickable':''}" style="grid-template-columns:${gc}" onClick=${()=>onRowClick&&onRowClick(row)}>${columns.map(c=>html`<span key=${c.key} class="${c.mono?'mono':''} ${c.accent?'dt-accent':''}">${c.render?c.render(row):row[c.key]||'\u2014'}</span>`)}</div>`)}</div></div>`;
}

