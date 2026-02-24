# LangSmith + Stockyard

Set environment variables:
```bash
export LANGCHAIN_TRACING_V2=true
export LANGCHAIN_API_KEY=ls-...
export LANGCHAIN_ENDPOINT=https://api.smith.langchain.com
```

Stockyard can export traces to LangSmith when using LangChain through the proxy.
For non-LangChain apps, use Stockyard's OTLP export instead.

