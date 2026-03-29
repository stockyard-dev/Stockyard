# Stockyard Desktop

Native desktop app for Stockyard using [Tauri](https://tauri.app/).

## Features

- **Menubar/tray icon** showing live cost ticker
- **Native window** opening the full Stockyard console
- **OS notifications** for cost alerts, provider failures, guardrail violations
- **Auto-start** Stockyard binary if not running
- **Auto-update** check on launch

## Prerequisites

- [Rust](https://rustup.rs/) (for Tauri)
- [Node.js](https://nodejs.org/) 18+
- Stockyard binary installed

## Development

```bash
cd desktop
npm install
npm run dev
```

## Build

### macOS
```bash
npm run build -- --target universal-apple-darwin
```

### Linux
```bash
npm run build -- --target x86_64-unknown-linux-gnu
```

### Windows
```bash
npm run build -- --target x86_64-pc-windows-msvc
```

## Configuration

On first launch, configure:
- **Stockyard URL** — defaults to `http://localhost:7749`
- **Admin Key** — for API access
- **Notifications** — toggle alerts for cost, provider, guardrail events

Settings stored in:
- macOS: `~/Library/Application Support/stockyard-desktop/config.json`
- Linux: `~/.config/stockyard-desktop/config.json`
- Windows: `%APPDATA%\stockyard-desktop\config.json`

## Architecture

```
desktop/
├── src/
│   ├── main.ts          # Tauri window + tray setup
│   ├── notifications.ts # OS notification handling
│   └── config.ts        # Settings management
├── src-tauri/
│   ├── Cargo.toml       # Rust dependencies
│   ├── src/
│   │   └── main.rs      # Tauri backend
│   └── tauri.conf.json  # Tauri configuration
├── package.json
└── README.md
```
