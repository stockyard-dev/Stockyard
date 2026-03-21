# stockyard-openai

Drop-in OpenAI SDK wrapper that routes all traffic through Stockyard.

## Installation

```bash
pip install stockyard-openai
```

## Usage

Change one import line:

```python
# Before
from openai import OpenAI

# After
from stockyard_openai import OpenAI
```

Everything else works exactly the same:

```python
client = OpenAI(api_key="sk-...")
response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "Hello"}]
)
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `STOCKYARD_URL` | `http://localhost:4200` | Stockyard proxy URL |
| `STOCKYARD_ENABLED` | `true` | Set to `false` to bypass Stockyard |

## How It Works

The wrapper subclasses `openai.OpenAI` and redirects the `base_url` to your Stockyard instance. All requests flow through Stockyard's middleware chain (caching, rate limiting, cost tracking, etc.) before reaching the provider.
