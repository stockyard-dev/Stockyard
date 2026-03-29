# Terraform Provider for Stockyard

Manage Stockyard infrastructure as code.

## Configuration

```hcl
provider "stockyard" {
  base_url  = "http://localhost:7749"  # or your Cloud URL
  admin_key = var.stockyard_admin_key
}
```

Environment variables: `STOCKYARD_URL`, `STOCKYARD_ADMIN_KEY`.

## Resources

### stockyard_module

Toggle middleware modules on/off with optional config:

```hcl
resource "stockyard_module" "costcap" {
  name        = "costcap"
  enabled     = true
  config_json = jsonencode({ daily_cap_usd = 50 })
}

resource "stockyard_module" "cache" {
  name    = "cache"
  enabled = true
}
```

### stockyard_provider

Manage LLM provider configurations:

```hcl
resource "stockyard_provider" "openai" {
  name     = "openai"
  api_key  = var.openai_api_key
  base_url = "https://api.openai.com/v1"
  enabled  = true
}

resource "stockyard_provider" "anthropic" {
  name     = "anthropic"
  api_key  = var.anthropic_api_key
  base_url = "https://api.anthropic.com"
  enabled  = true
}
```

### stockyard_routing_rule

Configure smart routing rules:

```hcl
resource "stockyard_routing_rule" "short_prompts" {
  name     = "short-to-mini"
  priority = 10
  condition = jsonencode({
    field = "prompt_length"
    op    = "<"
    value = 500
  })
  action = jsonencode({
    route_to_model = "gpt-4o-mini"
  })
}

resource "stockyard_routing_rule" "ab_test" {
  name     = "claude-ab-test"
  priority = 5
  condition = jsonencode({
    field = "ab_split"
    value = 50
  })
  action = jsonencode({
    route_to_model = "claude-3-haiku"
  })
}
```

### stockyard_alert

Create observe alert rules:

```hcl
resource "stockyard_alert" "high_cost" {
  name      = "daily-cost-warning"
  metric    = "cost_usd"
  condition = ">"
  threshold = 100
}

resource "stockyard_alert" "error_spike" {
  name      = "error-rate-alert"
  metric    = "error_rate"
  condition = ">"
  threshold = 0.1
}
```

### stockyard_team_member

Manage team invites and roles:

```hcl
resource "stockyard_team_member" "dev_lead" {
  email = "dev@company.com"
  name  = "Dev Lead"
  role  = "developer"
}

resource "stockyard_team_member" "auditor" {
  email = "compliance@company.com"
  name  = "Compliance"
  role  = "auditor"
}
```

## Data Sources

```hcl
data "stockyard_modules" "all" {}

data "stockyard_providers" "all" {}

data "stockyard_traces" "recent" {
  limit = 100
}

data "stockyard_status" "current" {}

output "uptime" {
  value = data.stockyard_status.current.uptime
}

output "module_count" {
  value = length(data.stockyard_modules.all.modules)
}
```

## Building

```bash
cd terraform-provider
go build -o terraform-provider-stockyard
```
