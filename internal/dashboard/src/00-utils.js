
const APPS={
  overview:{id:'overview',name:'Overview',icon:'\u25A3',desc:'System status'},
  proxy:{id:'proxy',name:'Chute',icon:'\u25C8',desc:'Proxy & middleware'},
  observe:{id:'observe',name:'Lookout',icon:'\u25CE',desc:'Traces & costs'},
  trust:{id:'trust',name:'Brand',icon:'\u2B21',desc:'Audit & compliance'},
  studio:{id:'studio',name:'Tack Room',icon:'\u25C7',desc:'Prompts & experiments'},
  forge:{id:'forge',name:'Forge',icon:'\u2B22',desc:'Workflows & tools'},
  exchange:{id:'exchange',name:'Trading Post',icon:'\u21C4',desc:'Pack marketplace'},
  products:{id:'products',name:'Products',icon:'\u2B22',desc:'All 29 products'},
};
const APP_ORDER=['overview','proxy','observe','trust','studio','forge','exchange','products'];
function ss(k,v){try{if(v===undefined)return sessionStorage.getItem(k)||'';if(v===null)sessionStorage.removeItem(k);else sessionStorage.setItem(k,v)}catch(e){return ''}}
let _adminKey=ss('sy_admin_key');
function setAdminKey(k){_adminKey=k;ss('sy_admin_key',k)}
let _teamScope=ss('sy_team_scope');
function setTeamScope(id){_teamScope=id;if(id)ss('sy_team_scope',id);else ss('sy_team_scope',null)}
function getTeamScope(){return _teamScope}
async function api(path,opts={}){
  const headers=opts.headers||{};
  if(_adminKey)headers['X-Admin-Key']=_adminKey;
  if(opts.body&&typeof opts.body==='string')headers['Content-Type']='application/json';
  let url=path;
  if(_teamScope&&(!opts.method||opts.method==='GET')){
    const sep=url.includes('?')?'&':'?';
    if(!url.includes('team_id='))url+=sep+'team_id='+_teamScope;
  }
  try{const r=await fetch(url,{...opts,headers});if(r.status===401||r.status===403)return{_error:r.status};if(!r.ok){const t=await r.text().catch(()=>'');try{return{_error:r.status,...JSON.parse(t)}}catch(e){return{_error:r.status,message:t}}}return await r.json()}catch(e){return{_error:e.message}}
}
const fmt={
  usd:v=>v==null?'\u2014':v<0.01&&v>0?'$'+v.toFixed(4):'$'+v.toFixed(2),
  num:v=>v==null?'\u2014':v>=1e6?(v/1e6).toFixed(1)+'M':v>=1e3?(v/1e3).toFixed(1)+'K':String(v),
  ms:v=>v==null?'\u2014':v>=1000?(v/1000).toFixed(1)+'s':Math.round(v)+'ms',
  ago:ts=>{if(!ts)return'\u2014';const s=Math.floor((Date.now()-new Date(ts))/1000);if(s<60)return s+'s ago';if(s<3600)return Math.floor(s/60)+'m ago';if(s<86400)return Math.floor(s/3600)+'h ago';return Math.floor(s/86400)+'d ago'},
  trunc:(s,n=60)=>s&&s.length>n?s.substring(0,n)+'\u2026':s||'\u2014',
  hash:s=>s?s.substring(0,12)+'\u2026':'\u2014',
};

