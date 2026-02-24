# Julep + Stockyard

Configure Julep to use Stockyard as the LLM provider:

```yaml
# julep config
llm:
  provider: openai
  base_url: http://stockyard:4000/v1
  api_key: any-string
```

