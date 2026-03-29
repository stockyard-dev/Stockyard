# stockyard-openai-go

Stockyard helpers for the Go OpenAI SDK.

## Installation

```bash
go get github.com/stockyard-dev/stockyard-openai-go
```

## Usage

```go
import (
    stockyard "github.com/stockyard-dev/stockyard-openai-go"
    openai "github.com/sashabaranov/go-openai"
)

config := openai.DefaultConfig(os.Getenv("OPENAI_API_KEY"))
config.BaseURL = stockyard.BaseURL()
config.HTTPClient = &http.Client{Transport: stockyard.Transport()}

client := openai.NewClientWithConfig(config)
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `STOCKYARD_URL` | `http://localhost:7749` | Stockyard proxy URL |
| `STOCKYARD_ENABLED` | `true` | Set to `false` to bypass Stockyard |
