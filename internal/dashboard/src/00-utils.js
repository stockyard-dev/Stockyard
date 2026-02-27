
const APPS={
  overview:{id:'overview',name:'Overview',icon:'\u25A3',desc:'System status'},
  proxy:{id:'proxy',name:'Proxy',icon:'\u25C8',desc:'Middleware & providers'},
  observe:{id:'observe',name:'Observe',icon:'\u25CE',desc:'Traces & costs'},
  trust:{id:'trust',name:'Trust',icon:'\u2B21',desc:'Policies & audit'},
  studio:{id:'studio',name:'Studio',icon:'\u25C7',desc:'Prompts & experiments'},
  forge:{id:'forge',name:'Forge',icon:'\u2B22',desc:'Workflows & tools'},
  exchange:{id:'exchange',name:'Exchange',icon:'\u21C4',desc:'Pack marketplace'},
};
const APP_ORDER=['overview','proxy','observe','trust','studio','forge','exchange'];
let _adminKey=sessionStorage.getItem('sy_admin_key')||'';
function setAdminKey(k){_adminKey=k;sessionStorage.setItem('sy_admin_key',k)}
async function api(path,opts={}){
  const headers=opts.headers||{};
  if(_adminKey)headers['X-Admin-Key']=_adminKey;
  if(opts.body&&typeof opts.body==='string')headers['Content-Type']='application/json';
  try{const r=await fetch(path,{...opts,headers});if(r.status===401||r.status===403)return{_error:r.status};if(!r.ok){const t=await r.text().catch(()=>'');try{return{_error:r.status,...JSON.parse(t)}}catch(e){return{_error:r.status,message:t}}}return await r.json()}catch(e){return{_error:e.message}}
}
const fmt={
  usd:v=>v==null?'\u2014':v<0.01&&v>0?'$'+v.toFixed(4):'$'+v.toFixed(2),
  num:v=>v==null?'\u2014':v>=1e6?(v/1e6).toFixed(1)+'M':v>=1e3?(v/1e3).toFixed(1)+'K':String(v),
  ms:v=>v==null?'\u2014':v>=1000?(v/1000).toFixed(1)+'s':Math.round(v)+'ms',
  ago:ts=>{if(!ts)return'\u2014';const s=Math.floor((Date.now()-new Date(ts))/1000);if(s<60)return s+'s ago';if(s<3600)return Math.floor(s/60)+'m ago';if(s<86400)return Math.floor(s/3600)+'h ago';return Math.floor(s/86400)+'d ago'},
  trunc:(s,n=60)=>s&&s.length>n?s.substring(0,n)+'\u2026':s||'\u2014',
  hash:s=>s?s.substring(0,12)+'\u2026':'\u2014',
};

