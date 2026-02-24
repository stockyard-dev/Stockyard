# W&B + Stockyard

Export Stockyard metrics to W&B for experiment tracking:

```python
import wandb
import requests

wandb.init(project="llm-proxy")

# Poll Stockyard stats and log to W&B
stats = requests.get("http://localhost:4000/api/stats").json()
wandb.log({
    "requests_total": stats["requests"],
    "cache_hit_rate": stats["cache_hit_rate"],
    "spend_today": stats["spend_today"],
    "avg_latency": stats["avg_latency_ms"],
})
```

