// ─── Teams (Team Key Isolation) ─────────────────────────────────────
function TeamsView(){
  const[teams,setTeams]=useState([]);const[sel,setSel]=useState(null);const[keys,setKeys]=useState([]);
  const[toast,setToast]=useState(null);const[showNew,setShowNew]=useState(false);const[showNewKey,setShowNewKey]=useState(false);
  const[newName,setNewName]=useState('');const[newDesc,setNewDesc]=useState('');const[keyName,setKeyName]=useState('');
  const[newKey,setNewKey]=useState(null);const[spend,setSpend]=useState(null);const[showEdit,setShowEdit]=useState(false);
  const[editName,setEditName]=useState('');const[editDesc,setEditDesc]=useState('');
  const loadTeams=async()=>{const r=await api('/api/teams');setTeams(r.teams||[])};
  useEffect(()=>{loadTeams()},[]);
  const selectTeam=async t=>{setSel(t);setNewKey(null);
    const[k,s]=await Promise.all([api('/api/teams/'+t.id+'/keys'),api('/api/teams/'+t.id+'/spend')]);
    setKeys(k.keys||[]);setSpend(s)};
  const createTeam=async()=>{const r=await api('/api/teams',{method:'POST',body:JSON.stringify({name:newName,description:newDesc})});
    if(r._error||r.error){setToast({msg:'Error: '+(r.error||r._error),type:'error'});return}
    setShowNew(false);setNewName('');setNewDesc('');setToast({msg:'Team created!',type:'success'});loadTeams();selectTeam(r)};
  const deleteTeam=async()=>{if(!sel||!confirm('Delete team "'+sel.name+'"? All team keys will be revoked.'))return;
    await api('/api/teams/'+sel.id,{method:'DELETE'});setSel(null);setKeys([]);setSpend(null);
    setToast({msg:'Team deleted',type:'success'});loadTeams()};
  const updateTeam=async()=>{if(!sel)return;const body={};if(editName)body.name=editName;if(editDesc!==undefined)body.description=editDesc;
    const r=await api('/api/teams/'+sel.id,{method:'PUT',body:JSON.stringify(body)});
    if(r._error||r.error){setToast({msg:'Error: '+(r.error||r._error),type:'error'});return}
    setShowEdit(false);setToast({msg:'Team updated',type:'success'});loadTeams();selectTeam(r)};
  const genKey=async()=>{if(!sel)return;const name=keyName||'key-'+(keys.length+1);
    const r=await api('/api/teams/'+sel.id+'/keys',{method:'POST',body:JSON.stringify({name})});
    if(r._error||r.error){setToast({msg:'Error: '+(r.error||r._error),type:'error'});return}
    setNewKey(r);setShowNewKey(false);setKeyName('');setToast({msg:'Key created!',type:'success'});selectTeam(sel)};
  const revoke=async kid=>{if(!sel||!confirm('Revoke this key?'))return;
    await api('/api/teams/'+sel.id+'/keys/'+kid,{method:'DELETE'});setToast({msg:'Revoked',type:'success'});selectTeam(sel)};
  const rotate=async kid=>{if(!sel||!confirm('Rotate this key? The old key will stop working immediately.'))return;
    const r=await api('/api/teams/'+sel.id+'/keys/'+kid+'/rotate',{method:'POST'});
    if(r.new_key)setNewKey(r.new_key);setToast({msg:'Key rotated',type:'success'});selectTeam(sel)};
  const fmtCost=v=>v!=null?'$'+v.toFixed(4):'$0.00';
  return html`<div class="page-head"><div class="page-eyebrow">Teams</div><h2>Team Key Isolation</h2><p class="page-sub">Create teams with separate API keys. Requests are isolated in logs, metrics, and spend tracking.</p></div>
    <div class="stats-row">
      <${Stat} label="Teams" value=${teams.length} accent/>
      <${Stat} label="Selected" value=${sel?sel.name:'None'}/>
      <${Stat} label="Team Keys" value=${sel?keys.length:'\u2014'}/>
      <${Stat} label="Team Spend (Month)" value=${spend?fmtCost(spend.this_month?.cost_usd):'\u2014'}/>
    </div>
    <div class="settings-layout">
      <div class="settings-sidebar"><div class="settings-sidebar-head"><span>Teams</span><${Btn} small variant="primary" onClick=${()=>setShowNew(true)}>+ New<//></div>
        <div class="user-list">${teams.map(t=>html`<div key=${t.id} class="user-item ${sel?.id===t.id?'active':''}" onClick=${()=>selectTeam(t)}>
          <div class="user-email">${t.name}</div>
          <div class="user-meta">${t.slug} \u00B7 ${fmt.ago(t.created_at)}</div>
        </div>`)}
        ${teams.length===0&&html`<div class="empty-state" style="padding:24px;font-size:0.8rem">No teams yet. Create one to get started.</div>`}</div></div>
      <div class="settings-main">
        ${!sel?html`<div class="empty-state">Select a team to manage keys and view usage.</div>`:html`
          <div class="section-head" style="display:flex;align-items:center;gap:12px;margin-bottom:16px">
            <h3 style="margin:0">${sel.name}</h3>
            <${Badge} text=${sel.slug} variant="muted"/>
            <div style="margin-left:auto;display:flex;gap:8px">
              <${Btn} small onClick=${()=>{setEditName(sel.name);setEditDesc(sel.description);setShowEdit(true)}}>Edit<//>
              <${Btn} small variant="danger" onClick=${deleteTeam}>Delete<//>
            </div>
          </div>
          ${sel.description&&html`<p style="color:var(--cream-dim);font-size:0.85rem;margin-bottom:16px">${sel.description}</p>`}

          ${spend&&html`<div class="stats-row" style="margin-bottom:20px">
            <${Stat} label="Today" value=${fmtCost(spend.today?.cost_usd)} sub=${(spend.today?.requests||0)+' reqs'}/>
            <${Stat} label="This Month" value=${fmtCost(spend.this_month?.cost_usd)} sub=${(spend.this_month?.requests||0)+' reqs'}/>
            <${Stat} label="All Time" value=${fmtCost(spend.total?.cost_usd)} sub=${(spend.total?.requests||0)+' reqs'}/>
            <${Stat} label="Total Tokens" value=${fmt.num((spend.total?.tokens_in||0)+(spend.total?.tokens_out||0))}/>
          </div>`}

          <div class="section-title">API Keys <${Btn} small onClick=${()=>setShowNewKey(true)}>+ Generate<//></div>
          ${newKey&&html`<div class="key-alert"><div class="key-alert-title">New API Key (copy now \u2014 shown once)</div><code class="key-alert-value">${newKey.key||newKey}</code>
            <${Btn} small onClick=${()=>{navigator.clipboard.writeText(newKey.key||newKey);setToast({msg:'Copied!',type:'success'})}}>Copy<//>
          </div>`}
          <${DataTable} columns=${[
            {key:'name',label:'Name',width:'1fr',mono:true},
            {key:'key_prefix',label:'Key',width:'1.5fr',mono:true},
            {key:'last_used',label:'Last Used',width:'100px',render:r=>fmt.ago(r.last_used)},
            {key:'a',label:'',width:'140px',render:r=>html`<div style="display:flex;gap:4px">
              <${Btn} small onClick=${()=>rotate(r.id)}>Rotate<//>
              <${Btn} small variant="danger" onClick=${()=>revoke(r.id)}>Revoke<//></div>`}
          ]} rows=${keys} emptyMsg="No API keys. Generate one to start isolating requests."/>

          <div class="section-title" style="margin-top:24px">Quick Start</div>
          <div style="background:var(--bg2);border:1px solid var(--bg3);padding:16px;margin-top:8px">
            <div style="font-size:0.78rem;color:var(--cream-dim);margin-bottom:8px">Use a team key just like a regular Stockyard key. Requests will be automatically attributed to this team.</div>
            <pre style="font-family:var(--font-mono);font-size:0.72rem;color:var(--leather-light);white-space:pre-wrap;margin:0">curl ${window.location.origin}/v1/chat/completions \\
  -H "Authorization: Bearer sk-sy-YOUR_TEAM_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}'</pre>
          </div>
        `}
      </div>
    </div>
    ${showNew&&html`<${Modal} title="Create Team" onClose=${()=>setShowNew(false)}>
      <${Input} label="Team Name" value=${newName} onChange=${setNewName} placeholder="e.g. Frontend, Backend, ML"/>
      <${Input} label="Description (optional)" value=${newDesc} onChange=${setNewDesc} placeholder="What this team is for"/>
      <div style="margin-top:16px;display:flex;gap:8px;justify-content:flex-end">
        <${Btn} onClick=${()=>setShowNew(false)}>Cancel<//>
        <${Btn} variant="primary" onClick=${createTeam} disabled=${!newName}>Create Team<//>
      </div>
    <//>`}
    ${showNewKey&&html`<${Modal} title="Generate Team Key" onClose=${()=>setShowNewKey(false)}>
      <${Input} label="Key Name" value=${keyName} onChange=${setKeyName} placeholder="e.g. production, staging, ci"/>
      <div style="margin-top:16px;display:flex;gap:8px;justify-content:flex-end">
        <${Btn} onClick=${()=>setShowNewKey(false)}>Cancel<//>
        <${Btn} variant="primary" onClick=${genKey}>Generate Key<//>
      </div>
    <//>`}
    ${showEdit&&html`<${Modal} title="Edit Team" onClose=${()=>setShowEdit(false)}>
      <${Input} label="Team Name" value=${editName} onChange=${setEditName}/>
      <${Input} label="Description" value=${editDesc} onChange=${setEditDesc}/>
      <div style="margin-top:16px;display:flex;gap:8px;justify-content:flex-end">
        <${Btn} onClick=${()=>setShowEdit(false)}>Cancel<//>
        <${Btn} variant="primary" onClick=${updateTeam}>Save Changes<//>
      </div>
    <//>`}
    ${toast&&html`<${Toast} msg=${toast.msg} type=${toast.type} onDone=${()=>setToast(null)}/>`}`;
}
