#!/usr/bin/env python3
"""Add FAQ sections and JSON-LD schema markup to money pages."""
import json, os

FAQ_STYLE = """
<style>
.faq-section{max-width:700px;margin:0 auto;padding:4rem 2rem 2rem}
.faq-section h2{font-family:var(--font-mono);font-size:0.75rem;letter-spacing:3px;text-transform:uppercase;color:var(--rust);margin-bottom:2rem;text-align:center}
.faq-item{border-bottom:1px solid var(--bg3);padding:1.2rem 0}
.faq-item:last-child{border-bottom:none}
.faq-q{font-family:var(--font-mono);font-size:0.88rem;color:var(--cream);margin-bottom:0.5rem;cursor:pointer}
.faq-q::before{content:'Q. ';color:var(--rust)}
.faq-a{font-size:0.85rem;color:var(--cream-dim);line-height:1.7}
</style>
"""

PAGES = {
    "proxy-only": {
        "faqs": [
            ("Can I use Stockyard as just a proxy without the other features?", "Yes. Proxy-only is a first-class use case. Install the binary, set your provider key, and route requests through one endpoint. Tracing, audit, and the other products are there if you want them later, but they are not required."),
            ("Does proxy-only mode cost anything?", "No. The Community tier is free and includes the full proxy with all 76 middleware modules, 40 provider integrations, caching, rate limiting, and cost tracking. No credit card required."),
            ("What providers does the proxy support?", "Stockyard supports 40 LLM providers including OpenAI, Anthropic, Google Gemini, Groq, Mistral, DeepSeek, Together AI, Fireworks, Ollama, and more. Set an environment variable and the provider is auto-configured."),
            ("Do I need Docker or Redis to run the proxy?", "No. Stockyard ships as a single Go binary with embedded SQLite. No Docker, no Redis, no Postgres, no external dependencies. Download and run."),
        ],
        "app_name": "Stockyard LLM Proxy",
        "app_desc": "Self-hosted LLM proxy with 76 middleware modules, 40 providers, caching, rate limiting, and cost tracking. One binary, zero dependencies.",
    },
    "model-aliasing": {
        "faqs": [
            ("What is LLM model aliasing?", "Model aliasing lets you define stable names for your app to use while mapping them to real provider models underneath. Your app calls 'fast' and Stockyard routes it to gpt-4o-mini, claude-haiku, or whatever you configure — changeable at runtime without redeploying."),
            ("Can I change aliases without restarting Stockyard?", "Yes. Aliases can be created, updated, and deleted through the REST API at runtime. Changes take effect on the next request with no restart or redeploy needed."),
            ("Does aliasing work across different providers?", "Yes. You can alias 'smart' to claude-sonnet-4-5 on Anthropic, then switch it to gpt-4o on OpenAI with one API call. Your application code never changes."),
            ("Is model aliasing free?", "Yes. Aliasing is included in the free Community tier along with all 76 middleware modules."),
        ],
        "app_name": "Stockyard Model Aliasing",
        "app_desc": "Map app-facing model names to real LLM providers. Swap GPT-4o for Claude or DeepSeek without changing application code. Runtime API, instant changes.",
    },
    "self-hosted-llm-proxy": {
        "faqs": [
            ("Why self-host an LLM proxy instead of using a SaaS?", "Self-hosting means your prompts, completions, and traces never leave your network. No third-party sees your data. You also avoid per-request SaaS pricing, which can add up at scale."),
            ("What infrastructure do I need to self-host Stockyard?", "Any machine that can run a Go binary. Stockyard is a single ~25MB file with embedded SQLite. No Redis, no Postgres, no Docker required. Runs on a VPS, bare metal, or Kubernetes."),
            ("Can I deploy Stockyard on Railway, Fly.io, or Render?", "Yes. Stockyard runs on any platform that supports Docker or Go binaries. It ships with a production Dockerfile and deploys to Railway, Fly.io, Render, or any container platform."),
            ("Is self-hosted Stockyard free?", "Yes. The Community tier is free forever with unlimited requests, all middleware modules, and all provider integrations. Paid tiers add specialized products, not capacity."),
        ],
        "app_name": "Stockyard Self-Hosted LLM Proxy",
        "app_desc": "Run your LLM proxy on your own infrastructure. Prompts and completions never leave your network. One binary, embedded SQLite, zero external services.",
    },
    "openai-compatible-proxy": {
        "faqs": [
            ("Is Stockyard compatible with the OpenAI API?", "Yes. Stockyard speaks the same /v1/chat/completions protocol as OpenAI. Change your base URL and your existing OpenAI SDK code works without modification."),
            ("Does it work with the Python OpenAI SDK?", "Yes. Set base_url to your Stockyard instance and api_key to your Stockyard key or provider key. Streaming, function calling, and all standard features work."),
            ("Can I use non-OpenAI models through the OpenAI API format?", "Yes. Stockyard includes shim modules that translate requests to Anthropic, Google Gemini, and other providers. Send an OpenAI-format request for claude-sonnet-4-5 and Stockyard handles the translation."),
            ("What tools and frameworks work with Stockyard?", "Any tool that supports the OpenAI API works: LangChain, Vercel AI SDK, LiteLLM, Instructor, Continue.dev, Cursor, and any other OpenAI-compatible client."),
        ],
        "app_name": "Stockyard OpenAI-Compatible Proxy",
        "app_desc": "OpenAI API-compatible LLM proxy. Change one base URL to route through 40 providers with caching, rate limiting, and cost tracking. Works with any OpenAI SDK.",
    },
    "why-sqlite": {
        "faqs": [
            ("Why does Stockyard use SQLite instead of Postgres?", "An LLM proxy is not a high-write web app. It handles tens to low hundreds of requests per second, and SQLite in WAL mode handles that comfortably. Using SQLite means zero external services, zero connection pool configuration, and zero database ops."),
            ("Can SQLite handle production traffic for an LLM proxy?", "Yes. SQLite in WAL mode supports concurrent reads with serialized writes. An LLM proxy bottleneck is always the upstream provider (1-30 second responses), not the database. SQLite handles the metadata writes easily."),
            ("How do I back up a SQLite-based proxy?", "Copy one file. That is the entire backup. No pg_dump, no point-in-time recovery configuration. Just cp stockyard.db backup.db."),
            ("When would I outgrow SQLite?", "If you need horizontal scaling across multiple proxy instances sharing state, or if you need to handle thousands of concurrent write-heavy operations beyond proxy request logging. For a single-instance proxy, SQLite handles millions of traced requests."),
        ],
        "app_name": "Stockyard",
        "app_desc": "Self-hosted LLM proxy using embedded SQLite. WAL mode, zero-config backups, no external database. One binary with 76 middleware modules and 40 providers.",
    },
}

def build_faq_html(faqs):
    items = []
    for q, a in faqs:
        items.append(f'    <div class="faq-item"><div class="faq-q">{q}</div><div class="faq-a">{a}</div></div>')
    return '<section class="faq-section">\n  <h2>Frequently Asked Questions</h2>\n' + '\n'.join(items) + '\n</section>'

def build_faq_schema(faqs, page_url):
    entries = []
    for q, a in faqs:
        entries.append({
            "@type": "Question",
            "name": q,
            "acceptedAnswer": {
                "@type": "Answer",
                "text": a,
            }
        })
    schema = {
        "@context": "https://schema.org",
        "@type": "FAQPage",
        "mainEntity": entries,
    }
    return '<script type="application/ld+json">\n' + json.dumps(schema, indent=2) + '\n</script>'

def build_app_schema(name, desc, page_url):
    schema = {
        "@context": "https://schema.org",
        "@type": "SoftwareApplication",
        "name": name,
        "description": desc,
        "applicationCategory": "DeveloperApplication",
        "operatingSystem": "Linux, macOS, Windows",
        "url": page_url,
        "offers": {
            "@type": "Offer",
            "price": "0",
            "priceCurrency": "USD",
        },
        "author": {
            "@type": "Organization",
            "name": "Stockyard",
            "url": "https://stockyard.dev",
        }
    }
    return '<script type="application/ld+json">\n' + json.dumps(schema, indent=2) + '\n</script>'

base = os.path.dirname(os.path.abspath(__file__))

for slug, page in PAGES.items():
    url = f"https://stockyard.dev/{slug}/"
    faq_html = build_faq_html(page["faqs"])
    faq_schema = build_faq_schema(page["faqs"], url)
    app_schema = build_app_schema(page["app_name"], page["app_desc"], url)

    insert_block = f"\n{faq_html}\n\n{faq_schema}\n{app_schema}\n"

    for d in ["site", os.path.join("internal", "site", "static")]:
        filepath = os.path.join(base, d, slug, "index.html")
        if not os.path.exists(filepath):
            print(f"  SKIP {filepath} (not found)")
            continue
        html = open(filepath).read()

        # Skip if already has FAQ schema
        if "FAQPage" in html:
            print(f"  SKIP {filepath} (already has FAQ schema)")
            continue

        # Insert FAQ style into <head> before </style> or </head>
        if FAQ_STYLE.strip() not in html:
            if "</style>" in html:
                # Find last </style>
                idx = html.rfind("</style>")
                html = html[:idx] + FAQ_STYLE.strip() + "\n</style>" + html[idx+len("</style>"):]
            elif "</head>" in html:
                html = html.replace("</head>", FAQ_STYLE + "</head>")

        # Insert FAQ section + schema before <footer>
        html = html.replace("<footer>", insert_block + "<footer>", 1)

        open(filepath, "w").write(html)
        print(f"  wrote {filepath}")

print(f"\nDone: {len(PAGES)} pages updated with FAQ sections + FAQPage + SoftwareApplication schema")
