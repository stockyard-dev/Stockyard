// ─── Settings (Users, API Keys, Provider Keys) ─────────────────────
function SettingsView(){
  const[users,setUsers]=useState([]);const[sel,setSel]=useState(null);const[keys,setKeys]=useState([]);const[provs,setProvs]=useState([]);
  const[toast,setToast]=useState(null);const[showNew,setShowNew]=useState(false);const[newEmail,setNewEmail]=useState('');const[newName,setNewName]=useState('');
  const[newKey,setNewKey]=useState(null);const[showProv,setShowProv]=useState(false);const[pn,setPn]=useState('openai');const[pk,setPk]=useState('');const[pu,setPu]=useState('');
  const loadUsers=async()=>{const r=await api('/api/auth/users');setUsers(r.users||[])};
  useEffect(()=>{loadUsers()},[]);
  const selectUser=async u=>{setSel(u);const[k,p]=await Promise.all([api('/api/auth/users/'+u.id+'/keys'),api('/api/auth/users/'+u.id+'/providers')]);setKeys(k.keys||[]);setProvs(p.providers||[])};
  const createUser=async()=>{const r=await api('/api/auth/users',{method:'POST',body:JSON.stringify({email:newEmail,name:newName})});if(r._error){setToast({msg:'Error: '+(r.error||r._error),type:'error'});return}setNewKey(r.api_key);setShowNew(false);setNewEmail('');setNewName('');setToast({msg:'User created!',type:'success'});loadUsers()};
  const genKey=async()=>{if(!sel)return;const r=await api('/api/auth/users/'+sel.id+'/keys',{method:'POST',body:JSON.stringify({name:'key-'+(keys.length+1)})});if(r._error){setToast({msg:'Error',type:'error'});return}setNewKey(r);setToast({msg:'Key created!',type:'success'});selectUser(sel)};
  const revoke=async kid=>{if(!sel)return;await api('/api/auth/users/'+sel.id+'/keys/'+kid,{method:'DELETE'});setToast({msg:'Revoked',type:'success'});selectUser(sel)};
  const addProv=async()=>{if(!sel||!pk)return;await api('/api/auth/users/'+sel.id+'/providers/'+pn,{method:'PUT',body:JSON.stringify({api_key:pk,base_url:pu})});setToast({msg:pn+' saved',type:'success'});setShowProv(false);setPk('');setPu('');selectUser(sel)};
  const delProv=async p=>{if(!sel)return;await api('/api/auth/users/'+sel.id+'/providers/'+p,{method:'DELETE'});setToast({msg:'Deleted',type:'success'});selectUser(sel)};
  return html`<div class="page-head"><div class="page-eyebrow">Settings</div><h2>Users & Configuration</h2><p class="page-sub">Manage users, API keys & provider credentials.</p></div>
    <div class="stats-row"><${Stat} label="Users" value=${users.length} accent/><${Stat} label="Selected" value=${sel?sel.email:'None'}/></div>
    <div class="settings-layout">
      <div class="settings-sidebar"><div class="settings-sidebar-head"><span>Users</span><${Btn} small variant="primary" onClick=${()=>setShowNew(true)}>+ New<//></div>
        <div class="user-list">${users.map(u=>html`<div key=${u.id} class="user-item ${sel?.id===u.id?'active':''}" onClick=${()=>selectUser(u)}><div class="user-email">${u.email}</div><div class="user-meta">${u.tier} \u00B7 ${fmt.ago(u.created_at)}</div></div>`)}
        ${users.length===0&&html`<div class="empty-state" style="padding:24px;font-size:0.8rem">No users.</div>`}</div></div>
      <div class="settings-main">
        ${!sel?html`<div class="empty-state">Select a user to manage keys and providers.</div>`:html`
          <div class="section-head"><h3>${sel.name||sel.email}</h3><${Badge} text=${sel.tier} variant=${sel.tier==='free'?'muted':'success'}/></div>
          <div class="section-title">API Keys <${Btn} small onClick=${genKey}>+ Generate<//></div>
          ${newKey&&html`<div class="key-alert"><div class="key-alert-title">New API Key (copy now \u2014 shown once)</div><code class="key-alert-value">${newKey.key||newKey}</code><${Btn} small onClick=${()=>{navigator.clipboard.writeText(newKey.key||newKey);setToast({msg:'Copied!',type:'success'})}}>Copy<//></div>`}
          <${DataTable} columns=${[{key:'name',label:'Name',width:'1fr',mono:true},{key:'key_prefix',label:'Key',width:'1.5fr',mono:true},{key:'last_used',label:'Last Used',width:'100px',render:r=>fmt.ago(r.last_used)},{key:'a',label:'',width:'80px',render:r=>html`<${Btn} small variant="danger" onClick=${()=>revoke(r.id)}>Revoke<//>`}]} rows=${keys} emptyMsg="No API keys."/>
          <div class="section-title" style="margin-top:24px">Provider Keys <${Btn} small onClick=${()=>setShowProv(true)}>+ Add<//></div>
          <${DataTable} columns=${[{key:'provider',label:'Provider',width:'1fr',mono:true},{key:'base_url',label:'Base URL',width:'1.5fr',mono:true,render:r=>r.base_url||'default'},{key:'ts',label:'Added',width:'100px',render:r=>fmt.ago(r.created_at)},{key:'a',label:'',width:'80px',render:r=>html`<${Btn} small variant="danger" onClick=${()=>delProv(r.provider)}>Delete<//>`}]} rows=${provs} emptyMsg="No provider keys."/>`}
      </div></div>
    ${showNew&&html`<${Modal} title="Create User" onClose=${()=>setShowNew(false)}><${Input} label="Email" value=${newEmail} onChange=${setNewEmail} placeholder="user@example.com"/><${Input} label="Name" value=${newName} onChange=${setNewName} placeholder="Optional"/><div style="margin-top:16px;display:flex;gap:8px;justify-content:flex-end"><${Btn} onClick=${()=>setShowNew(false)}>Cancel<//><${Btn} variant="primary" onClick=${createUser} disabled=${!newEmail}>Create User<//></div><//>`}
    ${showProv&&html`<${Modal} title="Add Provider Key" onClose=${()=>setShowProv(false)}><${Select} label="Provider" value=${pn} onChange=${setPn} options=${[{value:'openai',label:'OpenAI'},{value:'anthropic',label:'Anthropic'},{value:'gemini',label:'Google Gemini'},{value:'groq',label:'Groq'},{value:'together',label:'Together'},{value:'mistral',label:'Mistral'}]}/><${Input} label="API Key" value=${pk} onChange=${setPk} type="password" placeholder="sk-..." mono/><div style="margin-top:16px;display:flex;gap:8px;justify-content:flex-end"><${Btn} onClick=${()=>setShowProv(false)}>Cancel<//><${Btn} variant="primary" onClick=${addProv} disabled=${!pk}>Save<//></div><//>`}
    ${toast&&html`<${Toast} msg=${toast.msg} type=${toast.type} onDone=${()=>setToast(null)}/>`}`;
}

const VIEWS={overview:OverviewView,proxy:ProxyView,observe:ObserveView,trust:TrustView,studio:StudioView,forge:ForgeView,exchange:ExchangeView,settings:SettingsView};


