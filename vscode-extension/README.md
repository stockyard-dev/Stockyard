# Stockyard for VS Code

Control your Stockyard LLM proxy directly from VS Code.

## Features

- **Status bar indicator** — shows if Stockyard is running
- **Module toggle** — enable/disable middleware modules via quick pick
- **Trace viewer** — see recent proxy requests in a webview panel
- **Activity bar** — sidebar with modules, traces, and providers
- **Dashboard link** — open the web console with one click

## Setup

1. Install the extension
2. Start Stockyard: `stockyard` (or `docker compose up`)
3. The extension auto-connects to `http://localhost:4200`

## Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| `stockyard.baseUrl` | `http://localhost:4200` | Proxy URL |
| `stockyard.adminKey` | (empty) | Admin key for management API |
| `stockyard.autoRefresh` | `true` | Auto-refresh module and trace views |
| `stockyard.refreshInterval` | `5000` | Refresh interval in ms |

## Commands

- `Stockyard: Show Status` — system status panel
- `Stockyard: Toggle Module` — quick-pick module toggle
- `Stockyard: View Traces` — recent trace viewer
- `Stockyard: Open Dashboard` — open web console
- `Stockyard: Open Playground` — open playground
