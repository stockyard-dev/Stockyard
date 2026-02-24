# Arize / Phoenix + Stockyard

Use Stockyard's OTLP export to send traces to Arize:

```yaml
telemetry:
  otlp:
    endpoint: "https://otlp.arize.com"
    headers:
      space_key: "your-space-key"
      api_key: "your-api-key"
```

For Phoenix (open-source):
```yaml
telemetry:
  otlp:
    endpoint: "http://localhost:6006"
```

