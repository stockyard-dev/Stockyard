"""
Stockyard + OpenAI Python SDK

Just change the base_url — everything else works exactly the same.
Stockyard proxies the request, applies middleware (caching, cost tracking,
safety guardrails, etc.), and forwards to OpenAI.
"""
from openai import OpenAI

# Point at Stockyard instead of OpenAI directly
client = OpenAI(
    base_url="http://localhost:4200/v1",
    # api_key is still your OpenAI key — Stockyard passes it through
)

# Standard OpenAI SDK usage — zero code changes
response = client.chat.completions.create(
    model="gpt-4o-mini",
    messages=[
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "What is the capital of France?"},
    ],
)

print(response.choices[0].message.content)

# ── Multi-provider: just change the model ────────
# Stockyard routes based on model name or X-Provider header

# Anthropic (requires ANTHROPIC_API_KEY env var on Stockyard)
response = client.chat.completions.create(
    model="claude-sonnet-4-20250514",
    messages=[{"role": "user", "content": "Hello from Anthropic via Stockyard!"}],
)

# Groq (requires GROQ_API_KEY env var on Stockyard)
response = client.chat.completions.create(
    model="llama-3.1-70b-versatile",
    messages=[{"role": "user", "content": "Hello from Groq via Stockyard!"}],
)

# ── Check what happened ──────────────────────────
import requests

# View recent traces
traces = requests.get("http://localhost:4200/api/observe/traces?limit=5").json()
for t in traces.get("traces", []):
    print(f"  {t['model']} via {t['provider']} — {t['latency_ms']}ms, ${t.get('cost', 0):.4f}")

# View cost breakdown
costs = requests.get("http://localhost:4200/api/observe/costs").json()
print(f"\nTotal cost: ${costs.get('total_cost', 0):.4f}")
