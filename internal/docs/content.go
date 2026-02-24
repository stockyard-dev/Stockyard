package docs

// pageInstall returns the installation guide content.
func pageInstall() Page {
	return Page{
		Title:       "Installation",
		Path:        "install/index.html",
		Section:     "Getting Started",
		Description: "Install Stockyard on macOS, Linux, or Windows. Single binary, no dependencies.",
		Content: `<h1>Installation</h1>
<p class="lead">Single binary. No runtime dependencies. No Docker required. Just download and run.</p>

<div class="page-toc">
<div class="page-toc-title">On this page</div>
<ul>
<li><a href="#homebrew">Homebrew (macOS / Linux)</a></li>
<li><a href="#curl">Shell script</a></li>
<li><a href="#npm">npm / npx</a></li>
<li><a href="#go">Go install</a></li>
<li><a href="#docker">Docker</a></li>
<li><a href="#manual">Manual download</a></li>
<li><a href="#verify">Verify installation</a></li>
</ul>
</div>

<h2 id="homebrew">Homebrew (macOS / Linux)</h2>

<p>The fastest path if you already have Homebrew installed.</p>

<div class="code-title">Terminal</div>
<pre><code><span class="comment"># Add the Stockyard tap</span>
<span class="cmd">brew tap stockyard-dev/tap</span>

<span class="comment"># Install the full suite</span>
<span class="cmd">brew install stockyard</span>

<span class="comment"># Or install individual products</span>
<span class="cmd">brew install stockyard-dev/tap/costcap</span>
<span class="cmd">brew install stockyard-dev/tap/llmcache</span></code></pre>

<h2 id="curl">Shell script (macOS / Linux)</h2>

<p>One-line install that detects your OS and architecture automatically.</p>

<div class="code-title">Terminal</div>
<pre><code><span class="cmd">curl -fsSL https://get.stockyard.dev | sh</span></code></pre>

<p>This downloads the latest release binary for your platform and moves it to <code>/usr/local/bin</code>. Supports <code>x86_64</code> and <code>arm64</code> on both macOS and Linux.</p>

<p>To install a specific product instead of the suite:</p>

<div class="code-title">Terminal</div>
<pre><code><span class="cmd">curl -fsSL https://get.stockyard.dev | sh -s -- costcap</span></code></pre>

<h2 id="npm">npm / npx</h2>

<p>For JavaScript/TypeScript developers. The npm package is a thin wrapper that downloads the appropriate Go binary on first run.</p>

<div class="code-title">Terminal</div>
<pre><code><span class="comment"># Run directly with npx (downloads on first use)</span>
<span class="cmd">npx stockyard</span>

<span class="comment"># Or install globally</span>
<span class="cmd">npm install -g stockyard</span>

<span class="comment"># Individual products</span>
<span class="cmd">npx @stockyard/costcap</span>
<span class="cmd">npx @stockyard/llmcache</span></code></pre>

<h2 id="go">Go install</h2>

<p>If you have Go 1.22+ installed:</p>

<div class="code-title">Terminal</div>
<pre><code><span class="cmd">go install github.com/stockyard-dev/stockyard/cmd/stockyard@latest</span>

<span class="comment"># Individual products</span>
<span class="cmd">go install github.com/stockyard-dev/stockyard/cmd/costcap@latest</span></code></pre>

<h2 id="docker">Docker</h2>

<p>Official images on Docker Hub and GitHub Container Registry.</p>

<div class="code-title">Terminal</div>
<pre><code><span class="comment"># Full suite</span>
<span class="cmd">docker run -p 4000:4000 stockyard/stockyard</span>

<span class="comment"># Individual products</span>
<span class="cmd">docker run -p 4100:4100 stockyard/costcap</span>
<span class="cmd">docker run -p 4200:4200 stockyard/llmcache</span>

<span class="comment"># With config file</span>
<span class="cmd">docker run -p 4000:4000 \
  -v $(pwd)/config.yaml:/etc/stockyard/config.yaml \
  -e OPENAI_API_KEY=$OPENAI_API_KEY \
  stockyard/stockyard</span></code></pre>

<h2 id="manual">Manual download</h2>

<p>Download pre-built binaries from the <a href="https://github.com/stockyard-dev/stockyard/releases">GitHub Releases</a> page. Available for:</p>

<table>
<tr><th>OS</th><th>Architecture</th><th>File</th></tr>
<tr><td>macOS</td><td>Apple Silicon (arm64)</td><td><code>stockyard_darwin_arm64.tar.gz</code></td></tr>
<tr><td>macOS</td><td>Intel (amd64)</td><td><code>stockyard_darwin_amd64.tar.gz</code></td></tr>
<tr><td>Linux</td><td>x86_64</td><td><code>stockyard_linux_amd64.tar.gz</code></td></tr>
<tr><td>Linux</td><td>arm64</td><td><code>stockyard_linux_arm64.tar.gz</code></td></tr>
<tr><td>Windows</td><td>x86_64</td><td><code>stockyard_windows_amd64.zip</code></td></tr>
</table>

<p>Extract and move to your PATH:</p>

<div class="code-title">Terminal</div>
<pre><code><span class="cmd">tar xzf stockyard_darwin_arm64.tar.gz</span>
<span class="cmd">sudo mv stockyard /usr/local/bin/</span></code></pre>

<h2 id="verify">Verify installation</h2>

<div class="code-title">Terminal</div>
<pre><code><span class="cmd">stockyard --version</span>
<span class="output">stockyard v1.0.0 (darwin/arm64)</span>

<span class="cmd">stockyard --help</span></code></pre>

<div class="callout tip">
<span class="callout-label">Next step</span>
Continue to the <a href="/docs/quickstart/">Quickstart</a> to proxy your first LLM request in 30 seconds.
</div>`,
	}
}

// pageQuickstart returns the quickstart guide content.
func pageQuickstart() Page {
	return Page{
		Title:       "Quickstart",
		Path:        "quickstart/index.html",
		Section:     "Getting Started",
		Description: "Proxy your first LLM request through Stockyard in 30 seconds.",
		Content: `<h1>Quickstart</h1>
<p class="lead">From zero to proxied LLM request in 30 seconds. No config file needed.</p>

<h2 id="step1">1. Set your API key</h2>

<div class="code-title">Terminal</div>
<pre><code><span class="cmd">export OPENAI_API_KEY=sk-your-key-here</span></code></pre>

<p>Stockyard reads your provider keys from environment variables. It supports all major providers out of the box.</p>

<h2 id="step2">2. Start the proxy</h2>

<div class="code-title">Terminal</div>
<pre><code><span class="comment"># Start the full suite on port 4000</span>
<span class="cmd">stockyard</span>

<span class="comment"># Or start a specific product</span>
<span class="cmd">costcap</span>    <span class="comment"># Cost tracking on :4100</span>
<span class="cmd">llmcache</span>  <span class="comment"># Response caching on :4200</span></code></pre>

<p>You should see:</p>

<pre><code><span class="output">Stockyard is running
  Proxy:     http://localhost:4000/v1
  Dashboard: http://localhost:4000/ui
  Config:    default (no config file)</span></code></pre>

<h2 id="step3">3. Point your app at Stockyard</h2>

<p>Change your base URL from the provider to your local Stockyard instance. Every OpenAI-compatible SDK works.</p>

<div class="code-title">Python</div>
<pre><code><span class="keyword">from</span> openai <span class="keyword">import</span> OpenAI

client = OpenAI(
    base_url=<span class="string">"http://localhost:4000/v1"</span>,  <span class="comment"># Stockyard proxy</span>
    api_key=<span class="string">"sk-your-key"</span>,               <span class="comment"># passed through to provider</span>
)

response = client.chat.completions.create(
    model=<span class="string">"gpt-4o"</span>,
    messages=[{<span class="string">"role"</span>: <span class="string">"user"</span>, <span class="string">"content"</span>: <span class="string">"Hello"</span>}]
)</code></pre>

<div class="code-title">TypeScript</div>
<pre><code><span class="keyword">import</span> OpenAI <span class="keyword">from</span> <span class="string">"openai"</span>;

<span class="keyword">const</span> client = <span class="keyword">new</span> OpenAI({
  baseURL: <span class="string">"http://localhost:4000/v1"</span>,
  apiKey: <span class="string">"sk-your-key"</span>,
});

<span class="keyword">const</span> response = <span class="keyword">await</span> client.chat.completions.create({
  model: <span class="string">"gpt-4o"</span>,
  messages: [{ role: <span class="string">"user"</span>, content: <span class="string">"Hello"</span> }],
});</code></pre>

<div class="code-title">curl</div>
<pre><code><span class="cmd">curl http://localhost:4000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-your-key" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello"}]
  }'</span></code></pre>

<h2 id="step4">4. Open the dashboard</h2>

<p>Visit <a href="http://localhost:4000/ui">http://localhost:4000/ui</a> in your browser. You will see real-time metrics, request logs, and product-specific controls. The dashboard updates live via SSE.</p>

<h2 id="step5">5. Add a config file (optional)</h2>

<p>For production use, create a <code>config.yaml</code>:</p>

<div class="code-title">config.yaml</div>
<pre><code><span class="keyword">port</span>: 4000
<span class="keyword">providers</span>:
  - name: openai
    base_url: https://api.openai.com/v1
    api_key: ${OPENAI_API_KEY}
  - name: anthropic
    base_url: https://api.anthropic.com/v1
    api_key: ${ANTHROPIC_API_KEY}

<span class="keyword">costcap</span>:
  enabled: true
  daily_limit_usd: 10.00
  alert_threshold: 0.80

<span class="keyword">cache</span>:
  enabled: true
  ttl: 3600
  max_size_mb: 500</code></pre>

<div class="code-title">Terminal</div>
<pre><code><span class="cmd">stockyard --config config.yaml</span></code></pre>

<div class="callout tip">
<span class="callout-label">Next step</span>
See the <a href="/docs/config/">Configuration Reference</a> for all available options, or browse <a href="/docs/products/">individual product docs</a> for feature-specific guides.
</div>`,
	}
}

// pageConfig returns the configuration reference content.
func pageConfig() Page {
	return Page{
		Title:       "Configuration",
		Path:        "config/index.html",
		Section:     "Reference",
		Description: "Complete YAML configuration reference for all Stockyard products.",
		Content: `<h1>Configuration Reference</h1>
<p class="lead">All configuration is done through a single YAML file. Environment variables are interpolated at startup.</p>

<h2 id="basics">Basics</h2>

<p>Stockyard looks for configuration in this order:</p>
<ol>
<li><code>--config</code> flag: <code>stockyard --config /path/to/config.yaml</code></li>
<li><code>STOCKYARD_CONFIG</code> environment variable</li>
<li><code>./config.yaml</code> in the current directory</li>
<li><code>~/.config/stockyard/config.yaml</code></li>
<li>Default configuration (OpenAI provider, all features disabled)</li>
</ol>

<h2 id="env-vars">Environment variable interpolation</h2>

<p>Use <code>${VAR_NAME}</code> syntax anywhere in the config to reference environment variables. Supports defaults with <code>${VAR_NAME:-default}</code>.</p>

<pre><code><span class="keyword">providers</span>:
  - name: openai
    api_key: ${OPENAI_API_KEY}
    base_url: ${OPENAI_BASE_URL:-https://api.openai.com/v1}</code></pre>

<h2 id="global">Global settings</h2>

<table class="params">
<tr><th>Parameter</th><th>Type</th><th>Default</th><th>Description</th></tr>
<tr><td>port</td><td>int</td><td>4000</td><td>HTTP port for the proxy and dashboard</td></tr>
<tr><td>host</td><td>string</td><td>0.0.0.0</td><td>Bind address</td></tr>
<tr><td>log_level</td><td>string</td><td>info</td><td>Logging level: debug, info, warn, error</td></tr>
<tr><td>data_dir</td><td>string</td><td>./data</td><td>Directory for SQLite databases and cached data</td></tr>
<tr><td>admin_key</td><td>string</td><td></td><td>API key for management endpoints (<code>/api/*</code>)</td></tr>
</table>

<h2 id="providers">Providers</h2>

<p>Configure one or more LLM providers. The first provider is the default.</p>

<pre><code><span class="keyword">providers</span>:
  - name: openai
    base_url: https://api.openai.com/v1
    api_key: ${OPENAI_API_KEY}
    models:
      - gpt-4o
      - gpt-4o-mini
      - gpt-3.5-turbo

  - name: anthropic
    base_url: https://api.anthropic.com/v1
    api_key: ${ANTHROPIC_API_KEY}
    adapter: anthropic    <span class="comment"># Translate OpenAI format to Anthropic</span>
    models:
      - claude-sonnet-4-20250514

  - name: gemini
    base_url: https://generativelanguage.googleapis.com/v1beta
    api_key: ${GEMINI_API_KEY}
    adapter: gemini

  - name: groq
    base_url: https://api.groq.com/openai/v1
    api_key: ${GROQ_API_KEY}

  - name: ollama
    base_url: http://localhost:11434/v1
    models:
      - llama3
      - codellama</code></pre>

<table class="params">
<tr><th>Parameter</th><th>Type</th><th>Description</th></tr>
<tr><td>name</td><td>string</td><td>Unique identifier for this provider</td></tr>
<tr><td>base_url</td><td>string</td><td>API base URL (must include /v1 for OpenAI-compatible)</td></tr>
<tr><td>api_key</td><td>string</td><td>Provider API key (supports env var interpolation)</td></tr>
<tr><td>adapter</td><td>string</td><td>API format adapter: openai (default), anthropic, gemini</td></tr>
<tr><td>models</td><td>[]string</td><td>Allowed model names for this provider</td></tr>
<tr><td>timeout</td><td>duration</td><td>Request timeout (default: 120s)</td></tr>
<tr><td>max_retries</td><td>int</td><td>Max retry attempts (default: 0)</td></tr>
<tr><td>weight</td><td>int</td><td>Load balancing weight for round-robin (default: 1)</td></tr>
</table>

<h2 id="features">Feature configuration</h2>

<p>Each product/feature has its own config block. Disable any feature by setting <code>enabled: false</code> or omitting the block entirely.</p>

<h3>CostCap</h3>
<pre><code><span class="keyword">costcap</span>:
  enabled: true
  daily_limit_usd: 25.00
  monthly_limit_usd: 500.00
  alert_threshold: 0.80    <span class="comment"># Alert at 80% of limit</span>
  hard_cap: true            <span class="comment"># Block requests at 100%</span>
  per_model: true           <span class="comment"># Track per-model spend</span></code></pre>

<h3>CacheLayer</h3>
<pre><code><span class="keyword">cache</span>:
  enabled: true
  ttl: 3600                 <span class="comment"># Cache TTL in seconds</span>
  max_size_mb: 500
  semantic: true            <span class="comment"># Enable semantic similarity matching</span>
  semantic_threshold: 0.92  <span class="comment"># Cosine similarity threshold</span></code></pre>

<h3>RateShield</h3>
<pre><code><span class="keyword">rate_limit</span>:
  enabled: true
  requests_per_minute: 60
  tokens_per_minute: 100000
  per_key: true             <span class="comment"># Per API key limits</span>
  burst: 10                 <span class="comment"># Token bucket burst</span></code></pre>

<h3>FallbackRouter</h3>
<pre><code><span class="keyword">fallback</span>:
  enabled: true
  strategy: priority        <span class="comment"># priority, round-robin, least-latency</span>
  health_check_interval: 30 <span class="comment"># seconds</span>
  circuit_breaker:
    threshold: 5            <span class="comment"># failures before opening</span>
    timeout: 60             <span class="comment"># seconds before half-open</span></code></pre>

<h3>PromptGuard</h3>
<pre><code><span class="keyword">prompt_guard</span>:
  enabled: true
  mode: redact              <span class="comment"># redact or block</span>
  patterns:
    - email
    - phone
    - ssn
    - credit_card
  injection_detection: true</code></pre>

<p>See individual <a href="/docs/products/">product documentation</a> for the complete configuration options for each feature.</p>

<h2 id="license">License key</h2>

<pre><code><span class="keyword">license_key</span>: ${STOCKYARD_LICENSE_KEY}</code></pre>

<p>Or set via environment variable directly. The proxy reads <code>STOCKYARD_LICENSE_KEY</code> at startup. See <a href="/docs/license/">Licensing</a> for details.</p>

<h2 id="full-example">Full example</h2>

<div class="code-title">config.yaml</div>
<pre><code><span class="keyword">port</span>: 4000
<span class="keyword">host</span>: 0.0.0.0
<span class="keyword">log_level</span>: info
<span class="keyword">data_dir</span>: ./data
<span class="keyword">license_key</span>: ${STOCKYARD_LICENSE_KEY}

<span class="keyword">providers</span>:
  - name: openai
    base_url: https://api.openai.com/v1
    api_key: ${OPENAI_API_KEY}
  - name: anthropic
    base_url: https://api.anthropic.com/v1
    api_key: ${ANTHROPIC_API_KEY}
    adapter: anthropic

<span class="keyword">costcap</span>:
  enabled: true
  daily_limit_usd: 50.00

<span class="keyword">cache</span>:
  enabled: true
  ttl: 1800
  semantic: true

<span class="keyword">rate_limit</span>:
  enabled: true
  requests_per_minute: 120

<span class="keyword">fallback</span>:
  enabled: true
  strategy: priority</code></pre>`,
	}
}

// pageLicense returns the licensing documentation.
func pageLicense() Page {
	return Page{
		Title:       "Licensing",
		Path:        "license/index.html",
		Section:     "Reference",
		Description: "Stockyard licensing: free tier, paid tiers, offline validation, and key management.",
		Content: `<h1>Licensing</h1>
<p class="lead">Stockyard uses offline license keys validated with Ed25519 signatures. No phone-home. No license server. Works air-gapped.</p>

<h2 id="tiers">Tiers</h2>

<table>
<tr><th>Tier</th><th>Individual Product</th><th>Suite (125 products)</th><th>Limits</th></tr>
<tr><td>Free</td><td>$0</td><td>$0</td><td>1,000 requests/day, 5 products max (suite)</td></tr>
<tr><td>Starter</td><td>$9/mo</td><td>$19/mo</td><td>10,000 requests/day</td></tr>
<tr><td>Pro</td><td>$29/mo</td><td>$59/mo</td><td>100,000 requests/day</td></tr>
<tr><td>Team</td><td>$79/mo</td><td>$149/mo</td><td>Unlimited requests</td></tr>
<tr><td>Enterprise</td><td>Custom</td><td>$299+/mo</td><td>Custom terms, SLA, support</td></tr>
</table>

<div class="callout tip">
<span class="callout-label">Suite value</span>
The suite at $59/mo covers all 125 products. That is $0.47 per tool per month. Buying just 3 individual products at Pro tier costs more than the entire suite.
</div>

<h2 id="activation">Activation</h2>

<p>After purchase, you receive a license key starting with <code>SY-</code>. Set it as an environment variable:</p>

<div class="code-title">Terminal</div>
<pre><code><span class="comment"># Set for current session</span>
<span class="cmd">export STOCKYARD_LICENSE_KEY=SY-eyJwIjoic3RvY2t5...</span>

<span class="comment"># Add to shell profile for persistence</span>
<span class="cmd">echo 'export STOCKYARD_LICENSE_KEY=SY-eyJwIjoic3RvY2t5...' >> ~/.bashrc</span></code></pre>

<p>Or set it in your config file:</p>

<pre><code><span class="keyword">license_key</span>: ${STOCKYARD_LICENSE_KEY}</code></pre>

<p>The proxy validates the key at startup and unlocks the corresponding tier. No network call is made. Validation is purely cryptographic (Ed25519 signature verification).</p>

<h2 id="how-it-works">How it works</h2>

<p>License keys are self-contained signed tokens. The format is:</p>

<pre><code>SY-&lt;base64url(payload)&gt;.&lt;base64url(signature)&gt;</code></pre>

<p>The payload contains the product, tier, customer ID, issue date, and expiration date. The Ed25519 public key is embedded in every binary at build time. Validation takes microseconds and requires zero network access.</p>

<h2 id="key-management">Key management</h2>

<p>Use the <code>sy-keygen</code> CLI tool for key management:</p>

<div class="code-title">Terminal</div>
<pre><code><span class="comment"># Validate a key</span>
<span class="cmd">sy-keygen validate SY-eyJwIjoic3RvY2t5...</span>

<span class="comment"># Show key info (product, tier, expiry)</span>
<span class="cmd">sy-keygen info SY-eyJwIjoic3RvY2t5...</span></code></pre>

<h2 id="free-tier">Free tier</h2>

<p>Stockyard works without any license key. The free tier provides 1,000 requests per day with all features enabled. This is enough for local development and evaluation. When the daily limit is reached, the proxy returns <code>402 Payment Required</code> with a message indicating the limit.</p>

<h2 id="grace-period">Grace period</h2>

<p>If a subscription lapses, the license key continues working until its expiration date (typically 1 year from issue). After expiration, the proxy reverts to free tier limits. Configuration and data are preserved.</p>`,
	}
}

// pageDeploy returns the deployment guide content.
func pageDeploy() Page {
	return Page{
		Title:       "Deployment",
		Path:        "deploy/index.html",
		Section:     "Guides",
		Description: "Deploy Stockyard to production: Railway, Fly.io, Docker, AWS, and more.",
		Content: `<h1>Deployment</h1>
<p class="lead">Stockyard is a single static binary. It runs anywhere you can run an executable.</p>

<h2 id="railway">Railway (recommended for vibecoders)</h2>

<p>One-click deploy. Railway detects the Dockerfile and provisions everything.</p>

<div class="code-title">Terminal</div>
<pre><code><span class="comment"># From the Stockyard directory</span>
<span class="cmd">railway init</span>
<span class="cmd">railway up</span>

<span class="comment"># Set environment variables</span>
<span class="cmd">railway variables set OPENAI_API_KEY=sk-...</span>
<span class="cmd">railway variables set STOCKYARD_LICENSE_KEY=SY-...</span></code></pre>

<p>Or use the Railway template: <a href="https://railway.app/template/stockyard">Deploy to Railway</a>.</p>

<h2 id="fly">Fly.io</h2>

<div class="code-title">fly.toml</div>
<pre><code><span class="keyword">app</span> = <span class="string">"my-stockyard"</span>

[build]
  image = <span class="string">"stockyard/stockyard:latest"</span>

[env]
  PORT = <span class="string">"4000"</span>

[[services]]
  internal_port = 4000
  protocol = <span class="string">"tcp"</span>
  [[services.ports]]
    port = 443
    handlers = [<span class="string">"tls"</span>, <span class="string">"http"</span>]</code></pre>

<div class="code-title">Terminal</div>
<pre><code><span class="cmd">fly launch</span>
<span class="cmd">fly secrets set OPENAI_API_KEY=sk-...</span>
<span class="cmd">fly secrets set STOCKYARD_LICENSE_KEY=SY-...</span></code></pre>

<h2 id="docker-compose">Docker Compose</h2>

<div class="code-title">docker-compose.yml</div>
<pre><code><span class="keyword">services</span>:
  stockyard:
    image: stockyard/stockyard:latest
    ports:
      - <span class="string">"4000:4000"</span>
    volumes:
      - ./config.yaml:/etc/stockyard/config.yaml
      - stockyard-data:/data
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - STOCKYARD_LICENSE_KEY=${STOCKYARD_LICENSE_KEY}
    restart: unless-stopped

<span class="keyword">volumes</span>:
  stockyard-data:</code></pre>

<h2 id="systemd">Systemd (bare metal)</h2>

<div class="code-title">/etc/systemd/system/stockyard.service</div>
<pre><code>[Unit]
Description=Stockyard LLM Proxy
After=network.target

[Service]
Type=simple
User=stockyard
ExecStart=/usr/local/bin/stockyard --config /etc/stockyard/config.yaml
Restart=always
RestartSec=5
EnvironmentFile=/etc/stockyard/env

[Install]
WantedBy=multi-user.target</code></pre>

<div class="code-title">Terminal</div>
<pre><code><span class="cmd">sudo systemctl enable stockyard</span>
<span class="cmd">sudo systemctl start stockyard</span></code></pre>

<h2 id="kubernetes">Kubernetes / Helm</h2>

<div class="code-title">Terminal</div>
<pre><code><span class="cmd">helm repo add stockyard https://charts.stockyard.dev</span>
<span class="cmd">helm install stockyard stockyard/stockyard \
  --set env.OPENAI_API_KEY=sk-... \
  --set env.STOCKYARD_LICENSE_KEY=SY-...</span></code></pre>

<h2 id="terraform">Terraform (AWS)</h2>

<pre><code><span class="keyword">module</span> <span class="string">"stockyard"</span> {
  source  = <span class="string">"stockyard-dev/stockyard/aws"</span>
  version = <span class="string">"~> 1.0"</span>

  instance_type = <span class="string">"t3.small"</span>
  api_keys = {
    openai    = var.openai_api_key
    anthropic = var.anthropic_api_key
  }
}</code></pre>

<h2 id="production">Production checklist</h2>

<ul>
<li>Set <code>log_level: warn</code> to reduce noise</li>
<li>Configure <code>admin_key</code> to protect management endpoints</li>
<li>Mount a persistent volume for the <code>data_dir</code> (SQLite databases)</li>
<li>Set up your provider API keys as environment variables (not in config files)</li>
<li>Enable <code>costcap</code> with a daily limit as a safety net</li>
<li>Put behind a reverse proxy (nginx, Caddy) for TLS termination</li>
<li>Monitor with Prometheus (<code>/metrics</code> endpoint) or your preferred observability stack</li>
</ul>`,
	}
}

// pageAPIRef returns the API reference content.
func pageAPIRef() Page {
	return Page{
		Title:       "API Reference",
		Path:        "api/index.html",
		Section:     "Reference",
		Description: "Complete API reference for Stockyard proxy endpoints, management API, and dashboard SSE.",
		Content: `<h1>API Reference</h1>
<p class="lead">Stockyard exposes an OpenAI-compatible proxy API, a management API, and a real-time SSE endpoint.</p>

<div class="page-toc">
<div class="page-toc-title">On this page</div>
<ul>
<li><a href="#proxy">Proxy endpoints</a></li>
<li><a href="#management">Management API</a></li>
<li><a href="#dashboard">Dashboard &amp; SSE</a></li>
<li><a href="#headers">Custom headers</a></li>
<li><a href="#errors">Error handling</a></li>
</ul>
</div>

<h2 id="proxy">Proxy endpoints</h2>

<p>These endpoints are fully compatible with the OpenAI API specification. Point any OpenAI SDK at Stockyard and it works.</p>

<h3>Chat Completions</h3>
<p><span class="method post">POST</span> <code>/v1/chat/completions</code></p>

<p>Proxies chat completion requests. Supports streaming via SSE. This is the primary endpoint for most applications.</p>

<pre><code><span class="cmd">curl http://localhost:4000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-your-key" \
  -d '{
    "model": "gpt-4o",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Hello"}
    ],
    "stream": true
  }'</span></code></pre>

<h3>Completions (legacy)</h3>
<p><span class="method post">POST</span> <code>/v1/completions</code></p>

<p>Legacy completions endpoint for older models. Same proxy behavior as chat completions.</p>

<h3>Embeddings</h3>
<p><span class="method post">POST</span> <code>/v1/embeddings</code></p>

<p>Proxies embedding requests. Works with text-embedding-3-small, text-embedding-3-large, and all compatible models.</p>

<pre><code><span class="cmd">curl http://localhost:4000/v1/embeddings \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-your-key" \
  -d '{
    "model": "text-embedding-3-small",
    "input": "The quick brown fox"
  }'</span></code></pre>

<h2 id="management">Management API</h2>

<p>All management endpoints are at <code>/api/*</code>. Protected by <code>admin_key</code> if configured.</p>

<h3>Health check</h3>
<p><span class="method get">GET</span> <code>/health</code></p>
<p>Returns proxy status. No authentication required.</p>

<pre><code>{
  "status": "ok",
  "uptime": "4h23m",
  "version": "1.0.0",
  "features": ["costcap", "cache", "rate_limit", "fallback"]
}</code></pre>

<h3>Statistics</h3>
<p><span class="method get">GET</span> <code>/api/stats</code></p>
<p>Returns aggregate statistics across all enabled features.</p>

<pre><code>{
  "requests_total": 14892,
  "requests_today": 2341,
  "cache_hit_rate": 0.34,
  "total_spend_usd": 47.82,
  "active_providers": 2,
  "avg_latency_ms": 1240
}</code></pre>

<h3>Request log</h3>
<p><span class="method get">GET</span> <code>/api/requests</code></p>
<p>Returns recent request logs. Supports pagination with <code>?limit=</code> and <code>?offset=</code>.</p>

<h3>Configuration</h3>
<p><span class="method get">GET</span> <code>/api/config</code></p>
<p>Returns the active configuration (API keys redacted).</p>

<h3>License</h3>
<p><span class="method get">GET</span> <code>/api/license</code></p>
<p>Returns current license status, tier, and enforcement stats.</p>

<pre><code>{
  "valid": true,
  "product": "stockyard",
  "tier": "pro",
  "requests_today": 2341,
  "daily_limit": 100000,
  "expires_at": "2027-02-24T00:00:00Z"
}</code></pre>

<h2 id="dashboard">Dashboard and SSE</h2>

<h3>Dashboard UI</h3>
<p><span class="method get">GET</span> <code>/ui</code></p>
<p>Serves the embedded Preact dashboard. No separate build step required.</p>

<h3>Server-Sent Events</h3>
<p><span class="method get">GET</span> <code>/ui/events</code></p>
<p>Real-time event stream for the dashboard. Connect with <code>EventSource</code>:</p>

<pre><code><span class="keyword">const</span> es = <span class="keyword">new</span> EventSource(<span class="string">"/ui/events"</span>);
es.onmessage = (e) =&gt; {
  <span class="keyword">const</span> data = JSON.parse(e.data);
  <span class="comment">// data.type: "request", "stats", "alert", "cache_hit"</span>
};</code></pre>

<h2 id="headers">Custom headers</h2>

<p>Stockyard adds headers to proxied responses for observability:</p>

<table>
<tr><th>Header</th><th>Description</th></tr>
<tr><td><code>X-Stockyard-Cache</code></td><td><code>HIT</code> or <code>MISS</code></td></tr>
<tr><td><code>X-Stockyard-Provider</code></td><td>Provider name that served the request</td></tr>
<tr><td><code>X-Stockyard-Cost</code></td><td>Estimated cost in USD</td></tr>
<tr><td><code>X-Stockyard-Tokens</code></td><td>Total token count (prompt + completion)</td></tr>
<tr><td><code>X-Stockyard-Latency</code></td><td>Total latency in milliseconds</td></tr>
<tr><td><code>X-Stockyard-Request-Id</code></td><td>Unique request identifier</td></tr>
</table>

<p>You can use these headers in your application for cost tracking, logging, or conditional logic without parsing response bodies.</p>

<h2 id="errors">Error handling</h2>

<p>Stockyard returns standard OpenAI-compatible error responses:</p>

<pre><code>{
  "error": {
    "message": "Daily spend limit exceeded ($25.00/$25.00)",
    "type": "budget_exceeded",
    "code": 402
  }
}</code></pre>

<table>
<tr><th>Status</th><th>Meaning</th></tr>
<tr><td>400</td><td>Bad request (invalid JSON, missing fields)</td></tr>
<tr><td>401</td><td>Invalid or missing API key</td></tr>
<tr><td>402</td><td>License limit or spend cap exceeded</td></tr>
<tr><td>429</td><td>Rate limit exceeded</td></tr>
<tr><td>502</td><td>All providers failed (upstream error)</td></tr>
<tr><td>503</td><td>Circuit breaker open</td></tr>
</table>`,
	}
}
