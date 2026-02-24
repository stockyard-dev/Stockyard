# langchain-stockyard

> LangChain integration for Stockyard proxy.

```bash
pip install langchain-stockyard
```

```python
from langchain_stockyard import ChatStockyard

llm = ChatStockyard(model="gpt-4o-mini")
response = llm.invoke("What is Stockyard?")

# Streaming
for chunk in llm.stream("Tell me a story"):
    print(chunk.content, end="")

# Check spend
from langchain_stockyard import get_spend
print(get_spend())
```

Supports streaming, async, and all LangChain chain/agent patterns.

## License
MIT
