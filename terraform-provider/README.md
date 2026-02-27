# Terraform Provider for Stockyard

Manage Stockyard infrastructure as code.

## Status

**Stub implementation.** The API client and resource definitions are complete.
Full Terraform SDK integration is planned for a future release.

## Planned Resources

```hcl
# Toggle middleware modules
resource "stockyard_module" "costcap" {
  name    = "costcap"
  enabled = true
}

# Register webhooks
resource "stockyard_webhook" "slack_alerts" {
  url    = "https://hooks.slack.com/services/T00/B00/xxx"
  secret = var.webhook_secret
  events = "alert.fired,cost.threshold"
}

# Trust policies
resource "stockyard_trust_policy" "block_ssn" {
  name    = "block-ssn"
  type    = "content"
  action  = "block"
  pattern = "\\b\\d{3}-\\d{2}-\\d{4}\\b"
}
```

## Planned Data Sources

```hcl
# System status
data "stockyard_status" "current" {}

# Full config export
data "stockyard_config" "current" {}
```

## Configuration

```hcl
provider "stockyard" {
  base_url  = "http://localhost:4200"  # or your Cloud URL
  admin_key = var.stockyard_admin_key
}
```

## Building

```bash
cd terraform-provider
go build -o terraform-provider-stockyard
```
