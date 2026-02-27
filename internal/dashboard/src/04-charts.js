// ─── Chart Components ──────────────────────────────────────────────
function Sparkline({data,width=200,height=40,color='var(--rust-light)',fill=true}){
  if(!data||data.length<2)return html`<svg width=${width} height=${height}><text x=${width/2} y=${height/2+4} text-anchor="middle" fill="var(--cream-muted)" font-size="10" font-family="var(--font-mono)">No data</text></svg>`;
  const max=Math.max(...data,0.001);const min=Math.min(...data,0);const range=max-min||1;
  const pts=data.map((v,i)=>[i/(data.length-1)*width,(1-(v-min)/range)*(height-4)+2]);
  const line=pts.map((p,i)=>(i===0?'M':'L')+p[0].toFixed(1)+','+p[1].toFixed(1)).join(' ');
  const area=line+' L'+width+','+height+' L0,'+height+' Z';
  return html`<svg width=${width} height=${height} style="display:block">${fill&&html`<path d=${area} fill=${color} opacity="0.1"/>`}<path d=${line} fill="none" stroke=${color} stroke-width="1.5"/><circle cx=${pts[pts.length-1][0]} cy=${pts[pts.length-1][1]} r="2.5" fill=${color}/></svg>`;
}
function MiniBar({items,width=260,height=120}){
  if(!items||items.length===0)return html`<div class="empty-state" style="padding:16px;font-size:0.78rem">No data.</div>`;
  const max=Math.max(...items.map(i=>i.value),0.001);const barH=Math.min(18,Math.floor((height-8)/items.length)-4);
  return html`<svg width=${width} height=${items.length*(barH+6)+4} style="display:block">${items.map((item,i)=>{
    const w=Math.max(2,(item.value/max)*(width-90));const y=i*(barH+6)+2;
    return html`<g key=${i}><rect x="80" y=${y} width=${w} height=${barH} fill=${item.color||'var(--rust)'} rx="2" opacity="0.8"/><text x="76" y=${y+barH/2+4} text-anchor="end" fill="var(--cream-dim)" font-size="10" font-family="var(--font-mono)">${item.label}</text><text x=${82+w} y=${y+barH/2+4} fill="var(--cream-muted)" font-size="9" font-family="var(--font-mono)">${item.display||item.value.toFixed(2)}</text></g>`;
  })}</svg>`;
}

