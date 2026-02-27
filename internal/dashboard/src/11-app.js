function Onboarding({onComplete}){
  const[step,setStep]=useState(0);const[email,setEmail]=useState('');const[name,setName]=useState('');const[prov,setProv]=useState('openai');const[pk,setPk]=useState('');
  const[result,setResult]=useState(null);const[err,setErr]=useState('');const[loading,setLoading]=useState(false);
  const createUser=async()=>{setLoading(true);setErr('');const r=await api('/api/auth/users',{method:'POST',body:JSON.stringify({email,name})});setLoading(false);if(r._error||r.error){setErr(r.error||'Failed');return}setResult(r);setStep(1)};
  const addProvider=async()=>{if(!result?.user)return;setLoading(true);setErr('');const r=await api('/api/auth/users/'+result.user.id+'/providers/'+prov,{method:'PUT',body:JSON.stringify({api_key:pk})});setLoading(false);if(r._error||r.error){setErr(r.error||'Failed');return}setStep(2)};
  const finish=()=>{sessionStorage.setItem('sy_onboarded','1');onComplete()};
  const STEPS=[{n:'Create User',d:'Set up the first user account.'},{n:'Add Provider',d:'Connect your LLM provider API key.'},{n:'Ready',d:'Your proxy is configured.'}];
  return html`<div class="onboard-overlay"><div class="onboard-box">
    <div class="onboard-header"><div style="font-family:var(--font-mono);font-size:0.65rem;letter-spacing:3px;text-transform:uppercase;color:var(--leather);margin-bottom:8px">Welcome to Stockyard</div>
    <h2 style="font-size:1.4rem;margin-bottom:4px">Quick Setup</h2><p style="color:var(--cream-dim);font-style:italic;font-size:0.9rem">Get proxying in 2 minutes.</p></div>
    <div class="onboard-steps">${STEPS.map((s,i)=>html`<div key=${i} class="onboard-step ${i===step?'active':i<step?'done':''}"><span class="onboard-step-num">${i<step?'\u2713':i+1}</span><span>${s.n}</span></div>`)}</div>
    <div class="onboard-content">
    ${step===0?html`<${Input} label="Email" value=${email} onChange=${setEmail} placeholder="you@company.com"/>
      <${Input} label="Name (optional)" value=${name} onChange=${setName} placeholder="Your name"/>
      ${err&&html`<div style="color:var(--red);font-size:0.8rem;margin-bottom:8px">${err}</div>`}
      <${Btn} variant="primary" onClick=${createUser} disabled=${!email||loading}>${loading?'Creating...':'Create User & API Key'}<//>
      ${result&&html`<div class="key-alert" style="margin-top:12px"><div class="key-alert-title">Your API Key (copy now)</div><code class="key-alert-value">${result.api_key?.key}</code></div>`}`:
    step===1?html`<${Select} label="Provider" value=${prov} onChange=${setProv} options=${[{value:'openai',label:'OpenAI'},{value:'anthropic',label:'Anthropic'},{value:'gemini',label:'Google Gemini'},{value:'groq',label:'Groq'}]}/>
      <${Input} label="API Key" value=${pk} onChange=${setPk} type="password" placeholder="sk-..." mono/>
      ${err&&html`<div style="color:var(--red);font-size:0.8rem;margin-bottom:8px">${err}</div>`}
      <div style="display:flex;gap:8px"><${Btn} variant="primary" onClick=${addProvider} disabled=${!pk||loading}>${loading?'Saving...':'Save Provider Key'}<//><${Btn} onClick=${()=>setStep(2)}>Skip for now<//></div>`:
    html`<div style="text-align:center;padding:24px 0">
      <div style="font-size:2rem;margin-bottom:12px">\u2713</div>
      <p style="font-size:1rem;margin-bottom:8px">You\u2019re all set!</p>
      <p style="color:var(--cream-dim);font-size:0.85rem;margin-bottom:20px">Your proxy is ready at <code>/v1/chat/completions</code></p>
      <pre style="background:var(--bg);border:1px solid var(--bg3);padding:12px;font-family:var(--font-mono);font-size:0.75rem;text-align:left;color:var(--cream-dim);margin-bottom:20px">curl https://YOUR_HOST/v1/chat/completions \\\n  -H "Authorization: Bearer ${result?.api_key?.key||'sk-sy-...'}" \\\n  -H "Content-Type: application/json" \\\n  -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}'</pre>
      <${Btn} variant="primary" onClick=${finish}>Open Console<//></div>`}
    </div></div></div>`;
}

function AuthGate({onAuth}){
  const[key,setKey]=useState('');const[err,setErr]=useState('');
  const tryAuth=async()=>{setAdminKey(key);const r=await api('/api/proxy/modules');if(r._error===401||r._error===403){setErr('Invalid key');setAdminKey('');return}onAuth()};
  return html`<div style="display:flex;align-items:center;justify-content:center;min-height:100vh;background:var(--bg)"><div style="width:380px;padding:2rem;border:1px solid var(--bg3);background:var(--bg2)">
    <div style="font-family:var(--font-mono);font-size:0.65rem;letter-spacing:3px;text-transform:uppercase;color:var(--leather);margin-bottom:1.5rem;text-align:center">Stockyard Console</div>
    <div style="margin-bottom:1rem"><div style="font-family:var(--font-mono);font-size:0.7rem;color:var(--leather-light);margin-bottom:0.4rem">Admin Key</div>
    <input type="password" value=${key} onInput=${e=>setKey(e.target.value)} onKeyDown=${e=>e.key==='Enter'&&tryAuth()} style="width:100%;padding:0.6rem 0.8rem;background:var(--bg);border:1px solid var(--bg3);color:var(--cream);font-family:var(--font-mono);font-size:0.85rem;outline:none" placeholder="sy_admin_..."/></div>
    ${err&&html`<div style="font-family:var(--font-mono);font-size:0.75rem;color:var(--red);margin-bottom:0.8rem">${err}</div>`}
    <button onClick=${tryAuth} style="width:100%;padding:0.7rem;background:var(--rust);color:var(--cream);border:2px solid var(--rust);font-family:var(--font-mono);font-size:0.85rem;cursor:pointer">Authenticate</button>
  </div></div>`;
}

function App(){
  const[active,setActive]=useState('overview');const[authed,setAuthed]=useState(!!sessionStorage.getItem('sy_admin_key'));const[onboarded,setOnboarded]=useState(!!sessionStorage.getItem('sy_onboarded'));const[needsOnboard,setNeedsOnboard]=useState(false);const View=VIEWS[active]||OverviewView;
  useEffect(()=>{(async()=>{const r=await api('/api/proxy/modules');if(r._error===401||r._error===403){setAuthed(false);return}setAuthed(true);if(!sessionStorage.getItem('sy_onboarded')){const u=await api('/api/auth/users');if(!u.users||u.users.length===0){setNeedsOnboard(true)}}})()},[]);
  if(!authed)return html`<${AuthGate} onAuth=${()=>setAuthed(true)}/>`;
  const logout=()=>{setAdminKey('');setAuthed(false)};
  if(needsOnboard)return html`<${Onboarding} onComplete=${()=>setNeedsOnboard(false)}/>`;
  return html`<div class="shell"><nav class="nav">
    <div class="nav-brand" onClick=${()=>setActive('overview')}><span class="nav-brand-text">Stockyard</span></div>
    <div class="nav-divider"></div><div class="nav-section">Apps</div>
    ${APP_ORDER.map(id=>{const a=APPS[id];return html`<button key=${id} class="nav-item ${active===id?'active':''}" onClick=${()=>setActive(id)}><span class="nav-icon">${a.icon}</span><span class="nav-label">${a.name}</span></button>`})}
    <div class="nav-spacer"></div><div class="nav-divider"></div>
    <button class="nav-item ${active==='settings'?'active':''}" onClick=${()=>setActive('settings')}><span class="nav-icon">\u2699</span><span class="nav-label">Settings</span></button>
    <a class="nav-item" href="/api/apps" target="_blank"><span class="nav-icon">{}</span><span class="nav-label">Raw API</span></a>
    <a class="nav-item" href="https://stockyard.dev/docs" target="_blank"><span class="nav-icon">?</span><span class="nav-label">Docs</span></a>
    <button class="nav-item" onClick=${logout}><span class="nav-icon">\u23FB</span><span class="nav-label">Logout</span></button>
    <div class="nav-footer"><span class="nav-version">v1.0 \u00B7 6 apps \u00B7 69 endpoints</span></div>
  </nav><main class="content"><${View} key=${active}/></main></div>`;
}
render(html`<${App}/>`,document.getElementById('root'));
