# Retool Integration for Stockyard

Build internal AI dashboards with Stockyard as a REST API resource.

## Setup

### 1. Add REST API Resource
In Retool → Resources → Create New → REST API:
- **Base URL:** `https://your-stockyard-instance.com`
- **Headers:** `X-Admin-Key: your-admin-key`

### 2. Component Templates

**Module Toggle Panel:**
- Table component bound to `GET /api/proxy/modules` → `modules`
- Toggle column calling `PUT /api/proxy/modules/{{name}}` with `{"enabled": {{value}}}`

**Trace Explorer:**
- Table component bound to `GET /api/observe/traces?limit=50` → `traces`
- Columns: id, provider, model, status, duration_ms, cost_usd, created_at
- Detail panel showing full trace on row click

**Cost Dashboard:**
- Stat components bound to `GET /api/observe/costs?period=today`
- Chart component for daily cost trends from `GET /api/observe/costs/daily`

**Provider Health Grid:**
- Cards component bound to `GET /api/proxy/providers/health` → `providers`
- Conditional coloring: green=ok, yellow=degraded, red=unhealthy

**Team Management:**
- Table bound to `GET /api/team/members`
- Form for inviting: `POST /api/team/members` with email, name, role

## API Endpoints Reference
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/status` | GET | System status |
| `/api/proxy/modules` | GET | List modules |
| `/api/proxy/modules/{name}` | PUT | Toggle module |
| `/api/observe/traces` | GET | List traces |
| `/api/observe/costs` | GET | Cost breakdown |
| `/api/team/members` | GET/POST | Team management |
| `/api/billing/customers` | GET | Billing customers |
| `/api/firewall/stats` | GET | Firewall stats |
