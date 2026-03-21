# Make.com Integration for Stockyard

Pre-configured HTTP module templates for Make.com scenarios.

## Modules

### Send Prompt
- **HTTP Module** → POST `{{stockyard_url}}/v1/chat/completions`
- Headers: `Content-Type: application/json`, `Authorization: Bearer {{admin_key}}`
- Body: `{"model": "gpt-4o", "messages": [{"role": "user", "content": "{{message}}"}]}`

### List Traces
- **HTTP Module** → GET `{{stockyard_url}}/api/observe/traces?limit=20`
- Headers: `X-Admin-Key: {{admin_key}}`

### Toggle Module
- **HTTP Module** → PUT `{{stockyard_url}}/api/proxy/modules/{{module_name}}`
- Body: `{"enabled": true}`

### Check Costs
- **HTTP Module** → GET `{{stockyard_url}}/api/observe/costs?period=today`

### Provider Health
- **HTTP Module** → GET `{{stockyard_url}}/api/proxy/providers/health`

## Setup
1. Create a new Make.com scenario
2. Add an HTTP module for each action
3. Replace `{{stockyard_url}}` and `{{admin_key}}` with your values
