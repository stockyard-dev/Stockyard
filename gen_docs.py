#!/usr/bin/env python3
"""Generate docs pages from content. Usage: python3 gen_docs.py"""

import os, sys

SIDEBAR = """
    <div class="sidebar-section">
      <div class="sidebar-title">Getting Started</div>
      <a href="/docs/quickstart/"{{ACTIVE_quickstart}}>Quickstart</a>
      <a href="/docs/auth/"{{ACTIVE_auth}}>Auth &amp; Config</a>
      <a href="/docs/providers/"{{ACTIVE_providers}}>Providers</a>
      <a href="/docs/docker/"{{ACTIVE_docker}}>Docker</a>
    </div>
    <div class="sidebar-section">
      <div class="sidebar-title">Chute</div>
      <a href="/docs/proxy/"{{ACTIVE_proxy}}>Modules</a>
      <a href="/docs/aliasing/"{{ACTIVE_aliasing}}>Model Aliasing</a>
    </div>
    <div class="sidebar-section">
      <div class="sidebar-title">Apps</div>
      <a href="/docs/observe/"{{ACTIVE_observe}}>Lookout</a>
      <a href="/docs/trust/"{{ACTIVE_trust}}>Brand</a>
      <a href="/docs/studio/"{{ACTIVE_studio}}>Tack Room</a>
      <a href="/docs/forge/"{{ACTIVE_forge}}>Forge</a>
      <a href="/docs/team/"{{ACTIVE_team}}>Team</a>
      <a href="/docs/memory/"{{ACTIVE_memory}}>Memory</a>
      <a href="/docs/exchange/"{{ACTIVE_exchange}}>Trading Post</a>
    </div>
    <div class="sidebar-section">
      <div class="sidebar-title">Operations</div>
      <a href="/docs/ops/"{{ACTIVE_ops}}>Ops Guide</a>
      <a href="/docs/config/"{{ACTIVE_config}}>Configuration</a>
      <a href="/docs/railway-deploy/"{{ACTIVE_railway-deploy}}>Deploy</a>
    </div>
    <div class="sidebar-section">
      <div class="sidebar-title">Reference</div>
      <a href="/docs/api/"{{ACTIVE_api}}>API Reference</a>
      <a href="/docs/sdks/"{{ACTIVE_sdks}}>SDKs</a>
      <a href="/docs/integrations/"{{ACTIVE_integrations}}>Integrations</a>
      <a href="/docs/cli/"{{ACTIVE_cli}}>CLI</a>
    </div>"""

def gen(slug, title, description, content, prev_link="", prev_label="", next_link="", next_label=""):
    sidebar = SIDEBAR
    for s in ["quickstart","auth","providers","docker","proxy","aliasing","observe","trust",
              "studio","forge","team","memory","exchange","ops","config","railway-deploy",
              "api","sdks","integrations","cli"]:
        marker = "{{ACTIVE_" + s + "}}"
        sidebar = sidebar.replace(marker, ' class="active"' if s == slug else "")

    pn = ""
    if prev_link or next_link:
        pn = '<div class="pn">'
        if prev_link:
            pn += f'<a href="{prev_link}">&larr; {prev_label}</a>'
        if next_link:
            pn += f'<a class="next" href="{next_link}">{next_label} &rarr;</a>'
        pn += '</div>'

    return f'''<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<link rel="icon" type="image/x-icon" href="/site-assets/assets/brand/favicon.ico">
<link rel="icon" type="image/png" sizes="32x32" href="/site-assets/assets/brand/favicon-32.png">
<link rel="apple-touch-icon" href="/site-assets/assets/brand/apple-touch-icon.png">
<meta property="og:title" content="{title} — Stockyard">
<meta property="og:description" content="{description}">
<meta property="og:url" content="https://stockyard.dev/docs/{slug}/">
<meta property="og:type" content="website">
<meta property="og:image" content="https://stockyard.dev/site-assets/assets/marketing/og-card.png">
<meta name="twitter:card" content="summary_large_image">
<title>{title} — Stockyard Docs</title>
<meta name="description" content="{description}">
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=Libre+Baskerville:ital,wght@0,400;0,700;1,400&family=JetBrains+Mono:wght@400;600&display=swap" rel="stylesheet">
<style>
*{{margin:0;padding:0;box-sizing:border-box}}
:root{{
  --bg:#1a1410;--bg2:#241e18;--bg3:#2e261e;
  --rust:#c45d2c;--rust-light:#e8753a;
  --leather:#a0845c;--leather-light:#c4a87a;
  --cream:#f0e6d3;--cream-dim:#bfb5a3;--cream-muted:#b8a898;
  --gold:#d4a843;
  --green:#4a8c5c;--green-light:#5ba86e;
  --font-serif:'Libre Baskerville',Georgia,serif;
  --font-mono:'JetBrains Mono',monospace;
}}
.skip-link{{position:absolute;top:-100px;left:0;background:var(--rust);color:var(--cream);padding:0.5rem 1rem;z-index:9999;font-family:var(--font-mono);font-size:0.8rem;transition:top 0.2s}}.skip-link:focus{{top:0}}
body{{background:var(--bg);color:var(--cream);font-family:var(--font-serif);line-height:1.7;overflow-x:hidden}}
a{{color:var(--rust-light);text-decoration:none}}a:hover{{color:var(--gold)}}
.nav{{padding:1rem 2rem;display:flex;justify-content:space-between;align-items:center;border-bottom:1px solid var(--bg3)}}
.nav-brand{{font-family:var(--font-mono);font-size:0.9rem;color:var(--leather-light);letter-spacing:2px;text-transform:uppercase;display:flex;align-items:center;gap:10px}}
.nav-links{{display:flex;gap:1.5rem;font-size:0.85rem;font-family:var(--font-mono)}}
.nav-links a{{color:var(--cream-dim)}}.nav-links a:hover{{color:var(--rust-light)}}
.docs-layout{{display:grid;grid-template-columns:220px 1fr;max-width:1000px;margin:0 auto;min-height:calc(100vh - 60px)}}
.sidebar{{padding:2rem 1.5rem;border-right:1px solid var(--bg3);position:sticky;top:0;height:fit-content;max-height:100vh;overflow-y:auto}}
.sidebar-section{{margin-bottom:2rem}}
.sidebar-title{{font-family:var(--font-mono);font-size:0.65rem;letter-spacing:2px;text-transform:uppercase;color:var(--leather);margin-bottom:0.8rem}}
.sidebar a{{display:block;font-family:var(--font-mono);font-size:0.78rem;color:var(--cream-dim);padding:0.3rem 0;transition:color 0.15s}}
.sidebar a:hover,.sidebar a.active{{color:var(--rust-light)}}
.sidebar a.active{{border-left:2px solid var(--rust);padding-left:0.5rem}}
.content{{padding:3rem 2.5rem}}
.content h1{{font-size:2rem;margin-bottom:0.5rem}}
.content .subtitle{{font-size:1rem;color:var(--cream-dim);font-style:italic;margin-bottom:3rem}}
.content h2{{font-family:var(--font-mono);font-size:0.75rem;letter-spacing:3px;text-transform:uppercase;color:var(--rust);margin-top:3rem;margin-bottom:1rem;padding-top:2rem;border-top:1px solid var(--bg3)}}
.content h2:first-of-type{{margin-top:0;padding-top:0;border:none}}
.content h3{{font-size:1.1rem;margin-top:1.5rem;margin-bottom:0.5rem;color:var(--cream)}}
.content p{{color:var(--cream-dim);margin-bottom:1rem;line-height:1.8}}
.content code{{font-family:var(--font-mono);font-size:0.85em;background:var(--bg2);padding:0.15rem 0.4rem;color:var(--leather-light)}}
.content pre{{background:var(--bg2);border:1px solid var(--bg3);padding:1.2rem 1.5rem;font-family:var(--font-mono);font-size:0.8rem;color:var(--leather-light);overflow-x:auto;line-height:1.7;margin:1rem 0 1.5rem;position:relative}}
.copy-btn{{position:absolute;top:0.5rem;right:0.5rem;background:var(--bg3);border:1px solid var(--bg3);color:var(--cream-dim);font-family:var(--font-mono);font-size:0.6rem;padding:3px 8px;cursor:pointer;letter-spacing:1px;text-transform:uppercase;transition:all 0.2s;opacity:0}}
.copy-btn:hover{{background:var(--rust);border-color:var(--rust);color:var(--cream)}}
pre:hover .copy-btn{{opacity:1}}
.content .note{{background:var(--bg2);border-left:3px solid var(--gold);padding:1rem 1.2rem;margin:1.5rem 0;font-size:0.85rem;color:var(--cream-dim)}}
.content .note strong{{color:var(--gold);font-family:var(--font-mono);font-size:0.75rem;letter-spacing:1px;text-transform:uppercase}}
.endpoint-table{{width:100%;border-collapse:collapse;margin:1rem 0 1.5rem;font-size:0.8rem}}
.endpoint-table th,.endpoint-table td{{padding:0.5rem 0.8rem;text-align:left;border-bottom:1px solid var(--bg3)}}
.endpoint-table th{{font-family:var(--font-mono);font-size:0.65rem;letter-spacing:2px;text-transform:uppercase;color:var(--leather-light)}}
.endpoint-table td{{color:var(--cream-dim);font-family:var(--font-mono);font-size:0.75rem}}
.pn{{display:flex;justify-content:space-between;margin-top:3rem;padding-top:2rem;border-top:1px solid var(--bg3)}}
.pn a{{font-family:var(--font-mono);font-size:0.78rem;color:var(--cream-dim)}}.pn a:hover{{color:var(--rust-light)}}
.pn .next{{margin-left:auto}}
footer{{padding:2rem;text-align:center;font-family:var(--font-mono);font-size:0.75rem;color:var(--leather);border-top:1px solid var(--bg3)}}
footer .sig{{color:var(--leather-light);font-style:italic;font-family:var(--font-serif)}}
.nav-toggle{{display:none;background:none;border:none;cursor:pointer;padding:0.5rem;z-index:1001}}
.nav-toggle span{{display:block;width:22px;height:2px;background:var(--cream-dim);margin:5px 0;transition:all 0.3s}}
@media(max-width:700px){{.docs-layout{{grid-template-columns:1fr}}.sidebar{{display:none}}.content{{padding:2rem 1.5rem}}}}
</style>
<link rel="canonical" href="https://stockyard.dev/docs/{slug}/">
</head>
<body>
<a href="#main" class="skip-link">Skip to content</a>
<nav class="nav">
  <a href="/" class="nav-brand"><svg viewBox="0 0 64 64" width="24" height="24" fill="none"><rect x="8" y="8" width="8" height="48" rx="2.5" fill="#e8753a"/><rect x="28" y="8" width="8" height="48" rx="2.5" fill="#e8753a"/><rect x="48" y="8" width="8" height="48" rx="2.5" fill="#e8753a"/><rect x="8" y="27" width="48" height="7" rx="2.5" fill="#c4a87a"/></svg>Stockyard</a>
  <div class="nav-links">
    <a href="/products/">Products</a>
    <a href="/docs/">Docs</a>
    <a href="/benchmarks/">Benchmarks</a>
    <a href="/pricing/">Pricing</a>
    <a href="/playground/">Playground</a>
    <a href="https://github.com/stockyard-dev/Stockyard">GitHub</a>
  </div>
</nav>
<div class="docs-layout">
  <aside class="sidebar">{sidebar}
  </aside>
  <main class="content" id="main">
{content}

{pn}
  </main>
</div>
<footer><span class="sig">Stockyard</span> &mdash; One binary. Zero dependencies. Full control.</footer>

<script>
document.querySelectorAll('pre').forEach(pre => {{
  const btn = document.createElement('button');
  btn.className = 'copy-btn';
  btn.textContent = 'Copy';
  btn.onclick = () => {{
    const text = pre.textContent.replace(/^Copy/, '').trim();
    navigator.clipboard.writeText(text).then(() => {{
      btn.textContent = '\\u2713 Copied';
      setTimeout(() => btn.textContent = 'Copy', 2000);
    }});
  }};
  pre.appendChild(btn);
}});
</script>
</body>
</html>'''


# ────────────────── PAGE CONTENT ──────────────────

PAGES = {}

# ─── Team Docs (rewrite for key isolation) ───
PAGES["team"] = {
    "title": "Team Key Isolation",
    "description": "Create teams with isolated API keys, spend tracking, and request logs. Each team gets its own namespace for clean multi-project management.",
    "prev_link": "/docs/memory/", "prev_label": "Memory",
    "next_link": "/docs/exchange/", "next_label": "Trading Post",
    "content": """
<h1>Team Key Isolation</h1>
<p class="subtitle">Separate API keys, spend, and logs by team or project.</p>

<h2>Overview</h2>

<p>Teams let you carve Stockyard into isolated namespaces. Each team gets its own API keys, and every request made with a team key is automatically tagged in logs, traces, and spend tracking. No configuration needed beyond creating the team and generating keys.</p>

<p>This is the recommended way to give separate projects, environments, or teams their own slice of your Stockyard instance without running multiple instances.</p>

<h2>Create a Team</h2>

<pre>curl -X POST https://your-host/api/teams \\
  -H "X-Admin-Key: $ADMIN_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"name": "Frontend", "description": "Frontend web app"}'</pre>

<p>Response:</p>

<pre>{
  "id": 1,
  "name": "Frontend",
  "slug": "frontend",
  "description": "Frontend web app",
  "created_by": 0,
  "created_at": "2026-03-28T12:00:00Z",
  "updated_at": "2026-03-28T12:00:00Z"
}</pre>

<h2>Generate a Team Key</h2>

<pre>curl -X POST https://your-host/api/teams/1/keys \\
  -H "X-Admin-Key: $ADMIN_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"name": "production"}'</pre>

<p>The response includes the full API key. Copy it immediately &mdash; it is only shown once.</p>

<pre>{
  "id": 5,
  "user_id": 1,
  "team_id": 1,
  "key_prefix": "sk-sy-aBcDeF...",
  "name": "production",
  "scopes": "*",
  "key": "sk-sy-aBcDeFgHiJkLmNoPqRsTuVwXyZ..."
}</pre>

<h2>Use the Key</h2>

<p>Team keys work exactly like regular Stockyard keys. Just set the Authorization header:</p>

<pre>curl https://your-host/v1/chat/completions \\
  -H "Authorization: Bearer sk-sy-YOUR_TEAM_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello"}]
  }'</pre>

<p>Stockyard automatically detects the team key, tags the request with the team ID, and routes it through all configured middleware. Nothing else changes.</p>

<h2>View Team Spend</h2>

<pre>curl https://your-host/api/teams/1/spend \\
  -H "X-Admin-Key: $ADMIN_KEY"</pre>

<pre>{
  "team_id": "1",
  "total": {
    "requests": 1482,
    "cost_usd": 12.4830,
    "tokens_in": 892400,
    "tokens_out": 341200
  },
  "this_month": {
    "requests": 340,
    "cost_usd": 3.2100
  },
  "today": {
    "requests": 42,
    "cost_usd": 0.3800
  }
}</pre>

<h2>View Team Logs</h2>

<pre>curl "https://your-host/api/teams/1/logs?limit=20" \\
  -H "X-Admin-Key: $ADMIN_KEY"</pre>

<p>Returns only requests made with keys belonging to team 1. Supports <code>limit</code> and <code>offset</code> query parameters for pagination.</p>

<h2>Manage Keys</h2>

<table class="endpoint-table">
<tr><th>Action</th><th>Endpoint</th><th>Method</th></tr>
<tr><td>List keys</td><td><code>/api/teams/{id}/keys</code></td><td>GET</td></tr>
<tr><td>Create key</td><td><code>/api/teams/{id}/keys</code></td><td>POST</td></tr>
<tr><td>Revoke key</td><td><code>/api/teams/{id}/keys/{keyId}</code></td><td>DELETE</td></tr>
<tr><td>Rotate key</td><td><code>/api/teams/{id}/keys/{keyId}/rotate</code></td><td>POST</td></tr>
</table>

<p>Rotating a key atomically revokes the old key and generates a new one with the same name. The old key stops working immediately.</p>

<h2>Manage Teams</h2>

<table class="endpoint-table">
<tr><th>Action</th><th>Endpoint</th><th>Method</th></tr>
<tr><td>List teams</td><td><code>/api/teams</code></td><td>GET</td></tr>
<tr><td>Create team</td><td><code>/api/teams</code></td><td>POST</td></tr>
<tr><td>Get team + keys</td><td><code>/api/teams/{id}</code></td><td>GET</td></tr>
<tr><td>Update team</td><td><code>/api/teams/{id}</code></td><td>PUT</td></tr>
<tr><td>Delete team</td><td><code>/api/teams/{id}</code></td><td>DELETE</td></tr>
</table>

<p>Deleting a team automatically revokes all its keys.</p>

<h2>Dashboard</h2>

<p>The Teams tab in the dashboard (at <code>/ui</code>) provides a full UI for creating teams, generating keys, viewing spend, and managing everything without touching the API directly.</p>

<h2>How It Works</h2>

<p>When a request arrives with a team-scoped API key:</p>

<p>1. The auth middleware validates the key and looks up the associated team.</p>
<p>2. The team and user are injected into the request context.</p>
<p>3. The proxy handler copies the team ID onto the request metadata.</p>
<p>4. All downstream systems &mdash; request logging, trace recording, spend tracking &mdash; include the team ID automatically.</p>

<p>Existing user keys (without a team) continue to work exactly as before. Team isolation is purely additive.</p>

<div class="note">
<strong>Note:</strong> Team keys still require a user_id for ownership tracking. When creating a team key via the admin API, the key is owned by user ID 1 by default. You can specify a different <code>user_id</code> in the request body.
</div>
"""
}

# ─── Configuration Reference (new) ───
PAGES["config"] = {
    "title": "Configuration",
    "description": "Environment variables, runtime configuration, and tuning options for Stockyard.",
    "prev_link": "/docs/ops/", "prev_label": "Ops Guide",
    "next_link": "/docs/railway-deploy/", "next_label": "Deploy",
    "content": """
<h1>Configuration</h1>
<p class="subtitle">Environment variables and runtime options.</p>

<h2>Core</h2>

<table class="endpoint-table">
<tr><th>Variable</th><th>Default</th><th>Description</th></tr>
<tr><td><code>PORT</code></td><td><code>7749</code></td><td>HTTP server listen port</td></tr>
<tr><td><code>STOCKYARD_ADMIN_KEY</code></td><td><em>none</em></td><td>Admin API key. Required for all <code>/api/*</code> management endpoints. If unset, management API is open (dev mode only).</td></tr>
<tr><td><code>STOCKYARD_DB_PATH</code></td><td><code>./stockyard.db</code></td><td>Path to SQLite database file. Use a persistent volume in production.</td></tr>
<tr><td><code>STOCKYARD_REQUIRE_AUTH</code></td><td><code>false</code></td><td>When <code>true</code>, all <code>/v1/*</code> proxy requests require a valid <code>sk-sy-</code> API key.</td></tr>
<tr><td><code>STOCKYARD_TRUST_PROXY</code></td><td><em>none</em></td><td>When set, trusts <code>X-User-Id</code> and <code>X-Customer-ID</code> headers from upstream reverse proxies.</td></tr>
</table>

<h2>Provider Keys</h2>

<p>Configure LLM providers by setting environment variables. Stockyard auto-detects which providers are available based on which keys are set.</p>

<table class="endpoint-table">
<tr><th>Variable</th><th>Provider</th></tr>
<tr><td><code>OPENAI_API_KEY</code></td><td>OpenAI (GPT-4o, o1, o3)</td></tr>
<tr><td><code>ANTHROPIC_API_KEY</code></td><td>Anthropic (Claude)</td></tr>
<tr><td><code>GEMINI_API_KEY</code></td><td>Google Gemini</td></tr>
<tr><td><code>GROQ_API_KEY</code></td><td>Groq</td></tr>
<tr><td><code>MISTRAL_API_KEY</code></td><td>Mistral</td></tr>
<tr><td><code>TOGETHER_API_KEY</code></td><td>Together AI</td></tr>
<tr><td><code>DEEPSEEK_API_KEY</code></td><td>DeepSeek</td></tr>
<tr><td><code>FIREWORKS_API_KEY</code></td><td>Fireworks AI</td></tr>
<tr><td><code>PERPLEXITY_API_KEY</code></td><td>Perplexity</td></tr>
<tr><td><code>OPENROUTER_API_KEY</code></td><td>OpenRouter</td></tr>
<tr><td><code>XAI_API_KEY</code></td><td>xAI (Grok)</td></tr>
<tr><td><code>COHERE_API_KEY</code></td><td>Cohere</td></tr>
<tr><td><code>REPLICATE_API_TOKEN</code></td><td>Replicate</td></tr>
<tr><td><code>AZURE_OPENAI_API_KEY</code></td><td>Azure OpenAI</td></tr>
<tr><td><code>AZURE_OPENAI_ENDPOINT</code></td><td>Azure OpenAI endpoint URL</td></tr>
</table>

<p>Users can also store per-user provider keys via the API (<code>PUT /api/auth/me/providers/{provider}</code>). Per-user keys override global environment keys for that user's requests.</p>

<h2>Local Providers</h2>

<table class="endpoint-table">
<tr><th>Variable</th><th>Default</th><th>Description</th></tr>
<tr><td><code>OLLAMA_BASE_URL</code></td><td><code>http://localhost:11434</code></td><td>Ollama server URL</td></tr>
<tr><td><code>LMSTUDIO_BASE_URL</code></td><td><code>http://localhost:1234</code></td><td>LM Studio server URL</td></tr>
</table>

<h2>Billing</h2>

<table class="endpoint-table">
<tr><th>Variable</th><th>Description</th></tr>
<tr><td><code>STRIPE_SECRET_KEY</code></td><td>Stripe API key for subscription billing</td></tr>
<tr><td><code>STRIPE_WEBHOOK_SECRET</code></td><td>Stripe webhook signing secret</td></tr>
</table>

<h2>Persistence</h2>

<p>Stockyard stores everything in a single SQLite file. In production, mount a persistent volume at the database path:</p>

<pre># Docker
docker run -v stockyard-data:/data \\
  -e STOCKYARD_DB_PATH=/data/stockyard.db \\
  ghcr.io/stockyard-dev/stockyard</pre>

<pre># Railway
# Mount a volume at /data and set:
STOCKYARD_DB_PATH=/data/stockyard.db</pre>

<p>Back up by copying the file:</p>

<pre>cp /data/stockyard.db /data/backup-$(date +%Y%m%d).db</pre>

<h2>Runtime Configuration</h2>

<p>Most configuration beyond provider keys is managed at runtime through the API or dashboard. This includes middleware modules, routing rules, trust policies, spend caps, and alert rules. Changes take effect immediately without restart.</p>

<pre># Enable a middleware module
curl -X PUT https://your-host/api/proxy/modules/cache \\
  -H "X-Admin-Key: $ADMIN_KEY" \\
  -d '{"enabled": true}'

# Set a spend cap
curl -X POST https://your-host/api/config \\
  -H "X-Admin-Key: $ADMIN_KEY" \\
  -d '{"spend_caps": {"default": {"daily": 50.0, "monthly": 500.0}}}'</pre>

<h2>Production Checklist</h2>

<p>1. Set <code>STOCKYARD_ADMIN_KEY</code> to a strong random value.</p>
<p>2. Set <code>STOCKYARD_REQUIRE_AUTH</code>=true so proxy requests require API keys.</p>
<p>3. Mount a persistent volume for the SQLite database.</p>
<p>4. Set at least one provider API key.</p>
<p>5. Set up a reverse proxy (Caddy, nginx) for TLS termination.</p>
"""
}

# ─── SDKs (with real code) ───
PAGES["sdks"] = {
    "title": "SDKs",
    "description": "Use Stockyard from Python, Node.js, Go, or any OpenAI-compatible SDK. Drop-in replacement with one line change.",
    "prev_link": "/docs/api/", "prev_label": "API Reference",
    "next_link": "/docs/integrations/", "next_label": "Integrations",
    "content": """
<h1>SDKs</h1>
<p class="subtitle">Stockyard is OpenAI-compatible. Use any existing SDK.</p>

<h2>The One-Line Change</h2>

<p>Stockyard exposes the same <code>/v1/chat/completions</code> endpoint as OpenAI. To use it, point your existing SDK at your Stockyard instance and set your Stockyard API key. That's it.</p>

<h2>Python (OpenAI SDK)</h2>

<pre>from openai import OpenAI

client = OpenAI(
    base_url="https://your-host/v1",
    api_key="sk-sy-YOUR_KEY",
)

response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "Hello from Stockyard"}],
)
print(response.choices[0].message.content)</pre>

<p>Works with streaming too:</p>

<pre>stream = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "Count to 10"}],
    stream=True,
)
for chunk in stream:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="")</pre>

<h2>TypeScript / Node.js</h2>

<pre>import OpenAI from "openai";

const client = new OpenAI({
  baseURL: "https://your-host/v1",
  apiKey: "sk-sy-YOUR_KEY",
});

const response = await client.chat.completions.create({
  model: "gpt-4o",
  messages: [{ role: "user", content: "Hello from Stockyard" }],
});
console.log(response.choices[0].message.content);</pre>

<h2>Go</h2>

<pre>package main

import (
    "context"
    "fmt"
    openai "github.com/sashabaranov/go-openai"
)

func main() {
    config := openai.DefaultConfig("sk-sy-YOUR_KEY")
    config.BaseURL = "https://your-host/v1"
    client := openai.NewClientWithConfig(config)

    resp, err := client.CreateChatCompletion(
        context.Background(),
        openai.ChatCompletionRequest{
            Model: "gpt-4o",
            Messages: []openai.ChatCompletionMessage{
                {Role: "user", Content: "Hello from Stockyard"},
            },
        },
    )
    if err != nil {
        panic(err)
    }
    fmt.Println(resp.Choices[0].Message.Content)
}</pre>

<h2>curl</h2>

<pre>curl https://your-host/v1/chat/completions \\
  -H "Authorization: Bearer sk-sy-YOUR_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello"}]
  }'</pre>

<h2>Routing Headers</h2>

<p>Stockyard supports extra headers for routing and attribution that pass through any SDK:</p>

<pre># Python
response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "Hello"}],
    extra_headers={
        "X-Project": "my-app",
        "X-Provider": "anthropic",
        "X-Stockyard-Tags": "env=prod,team=backend",
    },
)</pre>

<pre>// TypeScript
const response = await client.chat.completions.create(
  {
    model: "gpt-4o",
    messages: [{ role: "user", content: "Hello" }],
  },
  {
    headers: {
      "X-Project": "my-app",
      "X-Provider": "anthropic",
      "X-Stockyard-Tags": "env=prod,team=backend",
    },
  }
);</pre>

<table class="endpoint-table">
<tr><th>Header</th><th>Description</th></tr>
<tr><td><code>X-Project</code></td><td>Project name for spend tracking (default: <code>default</code>)</td></tr>
<tr><td><code>X-Provider</code></td><td>Force a specific provider instead of auto-routing</td></tr>
<tr><td><code>X-Stockyard-Tags</code></td><td>Comma-separated key=value tags for cost attribution</td></tr>
<tr><td><code>X-Schema</code></td><td>JSON schema for structured output validation</td></tr>
</table>

<h2>Any OpenAI-Compatible SDK</h2>

<p>Stockyard works with any library that targets the OpenAI API: LiteLLM, LangChain, Vercel AI SDK, Instructor, and more. Just change the base URL and API key.</p>
"""
}

# ─── Integrations ───
PAGES["integrations"] = {
    "title": "Integrations",
    "description": "Use Stockyard with LangChain, Vercel AI SDK, LiteLLM, Instructor, and any OpenAI-compatible framework.",
    "prev_link": "/docs/sdks/", "prev_label": "SDKs",
    "next_link": "/docs/cli/", "next_label": "CLI",
    "content": """
<h1>Integrations</h1>
<p class="subtitle">Works with any framework that speaks OpenAI.</p>

<h2>LangChain (Python)</h2>

<pre>from langchain_openai import ChatOpenAI

llm = ChatOpenAI(
    base_url="https://your-host/v1",
    api_key="sk-sy-YOUR_KEY",
    model="gpt-4o",
)

response = llm.invoke("Explain LLM proxies in one sentence.")
print(response.content)</pre>

<h2>LangChain (TypeScript)</h2>

<pre>import { ChatOpenAI } from "@langchain/openai";

const llm = new ChatOpenAI({
  configuration: {
    baseURL: "https://your-host/v1",
    apiKey: "sk-sy-YOUR_KEY",
  },
  model: "gpt-4o",
});

const response = await llm.invoke("Hello from LangChain");
console.log(response.content);</pre>

<h2>Vercel AI SDK</h2>

<pre>import { createOpenAI } from "@ai-sdk/openai";
import { generateText } from "ai";

const stockyard = createOpenAI({
  baseURL: "https://your-host/v1",
  apiKey: "sk-sy-YOUR_KEY",
});

const { text } = await generateText({
  model: stockyard("gpt-4o"),
  prompt: "Hello from Vercel AI SDK",
});
console.log(text);</pre>

<h2>LiteLLM</h2>

<pre>import litellm

response = litellm.completion(
    model="openai/gpt-4o",
    messages=[{"role": "user", "content": "Hello from LiteLLM"}],
    api_key="sk-sy-YOUR_KEY",
    api_base="https://your-host/v1",
)
print(response.choices[0].message.content)</pre>

<h2>Instructor (Structured Output)</h2>

<pre>import instructor
from openai import OpenAI
from pydantic import BaseModel

client = instructor.from_openai(OpenAI(
    base_url="https://your-host/v1",
    api_key="sk-sy-YOUR_KEY",
))

class User(BaseModel):
    name: str
    age: int

user = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "John is 30 years old"}],
    response_model=User,
)
print(user)  # User(name='John', age=30)</pre>

<h2>Continue.dev (IDE)</h2>

<p>Add to your <code>~/.continue/config.json</code>:</p>

<pre>{
  "models": [
    {
      "title": "Stockyard GPT-4o",
      "provider": "openai",
      "model": "gpt-4o",
      "apiBase": "https://your-host/v1",
      "apiKey": "sk-sy-YOUR_KEY"
    }
  ]
}</pre>

<h2>Cursor / Windsurf / Cline</h2>

<p>Any AI code editor that supports custom OpenAI endpoints works with Stockyard. Set the base URL to <code>https://your-host/v1</code> and use your Stockyard API key. See <a href="/docs/editors/">Editor Setup</a> for per-editor instructions.</p>

<h2>Generic Pattern</h2>

<p>For any OpenAI-compatible library or tool:</p>

<p>1. Find the "base URL" or "API base" setting.</p>
<p>2. Set it to <code>https://your-stockyard-host/v1</code>.</p>
<p>3. Use your Stockyard API key (<code>sk-sy-...</code>) as the API key.</p>
<p>4. Use any model name your configured providers support.</p>

<p>Stockyard handles provider routing, failover, caching, and all middleware transparently. The framework never knows it's not talking directly to OpenAI.</p>
"""
}


# ────────────────── GENERATE ──────────────────

if __name__ == "__main__":
    base = os.path.dirname(os.path.abspath(__file__))
    site_dir = os.path.join(base, "site", "docs")
    static_dir = os.path.join(base, "internal", "site", "static", "docs")

    for slug, page in PAGES.items():
        html = gen(
            slug=slug,
            title=page["title"],
            description=page["description"],
            content=page["content"],
            prev_link=page.get("prev_link", ""),
            prev_label=page.get("prev_label", ""),
            next_link=page.get("next_link", ""),
            next_label=page.get("next_label", ""),
        )

        for d in [site_dir, static_dir]:
            out_dir = os.path.join(d, slug)
            os.makedirs(out_dir, exist_ok=True)
            out_path = os.path.join(out_dir, "index.html")
            with open(out_path, "w") as f:
                f.write(html)
            print(f"  wrote {out_path} ({len(html)} bytes)")

    print(f"\nGenerated {len(PAGES)} docs pages (site/ + internal/site/static/)")
