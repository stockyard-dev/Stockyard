# Composio + Stockyard

Composio provides 150+ tool integrations for AI agents.
Route the LLM calls through Stockyard:

```python
from openai import OpenAI
from composio_openai import ComposioToolSet

client = OpenAI(base_url="http://localhost:4000/v1", api_key="any")
toolset = ComposioToolSet()
```

Stockyard handles cost/caching/routing. Composio handles tools.

