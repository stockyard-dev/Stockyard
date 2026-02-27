import * as vscode from 'vscode';
import * as http from 'http';

let statusBarItem: vscode.StatusBarItem;

export function activate(context: vscode.ExtensionContext) {
  const config = () => vscode.workspace.getConfiguration('stockyard');
  const baseUrl = () => config().get<string>('baseUrl', 'http://localhost:4200');

  // Status bar
  statusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 50);
  statusBarItem.command = 'stockyard.showStatus';
  statusBarItem.text = '$(server) Stockyard';
  statusBarItem.tooltip = 'Click for Stockyard status';
  statusBarItem.show();
  context.subscriptions.push(statusBarItem);

  // Periodic health check
  const checkHealth = async () => {
    try {
      const data = await apiGet(baseUrl(), '/health');
      if (data?.status === 'ok') {
        statusBarItem.text = '$(server) Stockyard ✓';
        statusBarItem.backgroundColor = undefined;
      } else {
        statusBarItem.text = '$(server) Stockyard ⚠';
        statusBarItem.backgroundColor = new vscode.ThemeColor('statusBarItem.warningBackground');
      }
    } catch {
      statusBarItem.text = '$(server) Stockyard ✗';
      statusBarItem.backgroundColor = new vscode.ThemeColor('statusBarItem.errorBackground');
    }
  };

  checkHealth();
  const interval = setInterval(checkHealth, config().get<number>('refreshInterval', 5000));
  context.subscriptions.push({ dispose: () => clearInterval(interval) });

  // Commands
  context.subscriptions.push(
    vscode.commands.registerCommand('stockyard.showStatus', async () => {
      try {
        const status = await apiGet(baseUrl(), '/api/status');
        const panel = vscode.window.createWebviewPanel('stockyard.status', 'Stockyard Status', vscode.ViewColumn.One);
        panel.webview.html = renderStatusHTML(status);
      } catch (err: any) {
        vscode.window.showErrorMessage(`Stockyard: ${err.message}`);
      }
    }),

    vscode.commands.registerCommand('stockyard.toggleModule', async () => {
      try {
        const modules = await apiGet(baseUrl(), '/api/proxy/modules');
        const items = (modules.modules || []).map((m: any) => ({
          label: `${m.enabled ? '$(check)' : '$(circle-slash)'} ${m.name}`,
          description: m.enabled ? 'enabled' : 'disabled',
          name: m.name,
          enabled: m.enabled,
        }));
        const selected = await vscode.window.showQuickPick(items, { placeHolder: 'Toggle a module' });
        if (selected) {
          await apiPut(baseUrl(), `/api/proxy/modules/${selected.name}`, { enabled: !selected.enabled });
          vscode.window.showInformationMessage(`${selected.name}: ${!selected.enabled ? 'enabled' : 'disabled'}`);
        }
      } catch (err: any) {
        vscode.window.showErrorMessage(`Stockyard: ${err.message}`);
      }
    }),

    vscode.commands.registerCommand('stockyard.viewTraces', async () => {
      try {
        const traces = await apiGet(baseUrl(), '/api/observe/traces?limit=20');
        const panel = vscode.window.createWebviewPanel('stockyard.traces', 'Stockyard Traces', vscode.ViewColumn.One);
        panel.webview.html = renderTracesHTML(traces);
      } catch (err: any) {
        vscode.window.showErrorMessage(`Stockyard: ${err.message}`);
      }
    }),

    vscode.commands.registerCommand('stockyard.openDashboard', () => {
      vscode.env.openExternal(vscode.Uri.parse(`${baseUrl()}/ui`));
    }),

    vscode.commands.registerCommand('stockyard.openPlayground', () => {
      vscode.env.openExternal(vscode.Uri.parse(`${baseUrl()}/playground`));
    }),
  );

  // Tree views
  const modulesProvider = new ModulesTreeProvider(baseUrl);
  const tracesProvider = new TracesTreeProvider(baseUrl);
  vscode.window.registerTreeDataProvider('stockyard.modules', modulesProvider);
  vscode.window.registerTreeDataProvider('stockyard.traces', tracesProvider);

  if (config().get<boolean>('autoRefresh', true)) {
    const refreshInterval = setInterval(() => {
      modulesProvider.refresh();
      tracesProvider.refresh();
    }, config().get<number>('refreshInterval', 5000));
    context.subscriptions.push({ dispose: () => clearInterval(refreshInterval) });
  }
}

export function deactivate() {
  statusBarItem?.dispose();
}

// ── API helpers ──────────────────────────────────

function apiGet(base: string, path: string): Promise<any> {
  return new Promise((resolve, reject) => {
    const url = new URL(path, base);
    http.get(url.toString(), { timeout: 3000 }, (res) => {
      let data = '';
      res.on('data', (chunk) => data += chunk);
      res.on('end', () => {
        try { resolve(JSON.parse(data)); }
        catch { resolve({ raw: data }); }
      });
    }).on('error', reject);
  });
}

function apiPut(base: string, path: string, body: any): Promise<any> {
  return new Promise((resolve, reject) => {
    const url = new URL(path, base);
    const payload = JSON.stringify(body);
    const req = http.request(url.toString(), {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(payload) },
      timeout: 3000,
    }, (res) => {
      let data = '';
      res.on('data', (chunk) => data += chunk);
      res.on('end', () => {
        try { resolve(JSON.parse(data)); }
        catch { resolve({ raw: data }); }
      });
    });
    req.on('error', reject);
    req.write(payload);
    req.end();
  });
}

// ── Tree Providers ───────────────────────────────

class ModulesTreeProvider implements vscode.TreeDataProvider<vscode.TreeItem> {
  private _onDidChangeTreeData = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this._onDidChangeTreeData.event;
  constructor(private baseUrl: () => string) {}

  refresh() { this._onDidChangeTreeData.fire(); }

  async getChildren(): Promise<vscode.TreeItem[]> {
    try {
      const data = await apiGet(this.baseUrl(), '/api/proxy/modules');
      return (data.modules || []).map((m: any) => {
        const item = new vscode.TreeItem(m.name);
        item.description = m.enabled ? '✓ enabled' : '✗ disabled';
        item.iconPath = new vscode.ThemeIcon(m.enabled ? 'pass' : 'circle-slash');
        item.contextValue = 'module';
        return item;
      });
    } catch {
      return [new vscode.TreeItem('Stockyard not running')];
    }
  }

  getTreeItem(item: vscode.TreeItem) { return item; }
}

class TracesTreeProvider implements vscode.TreeDataProvider<vscode.TreeItem> {
  private _onDidChangeTreeData = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this._onDidChangeTreeData.event;
  constructor(private baseUrl: () => string) {}

  refresh() { this._onDidChangeTreeData.fire(); }

  async getChildren(): Promise<vscode.TreeItem[]> {
    try {
      const data = await apiGet(this.baseUrl(), '/api/observe/traces?limit=10');
      return (data.traces || []).map((t: any) => {
        const item = new vscode.TreeItem(`${t.model} — ${t.latency_ms}ms`);
        item.description = `${t.tokens_total || '?'} tokens`;
        item.tooltip = `Provider: ${t.provider}\nCost: $${t.cost || '0.00'}\nStatus: ${t.status}`;
        item.iconPath = new vscode.ThemeIcon(t.status === 'ok' ? 'pass' : 'error');
        return item;
      });
    } catch {
      return [new vscode.TreeItem('No traces yet')];
    }
  }

  getTreeItem(item: vscode.TreeItem) { return item; }
}

// ── Webview HTML ─────────────────────────────────

function renderStatusHTML(status: any): string {
  const comps = Object.entries(status.components || {}).map(([k, v]: [string, any]) =>
    `<tr><td>${k}</td><td style="color:${v.status === 'healthy' ? '#5ba86e' : '#d4a843'}">${v.status}</td><td>${v.detail || ''}</td></tr>`
  ).join('');

  return `<!DOCTYPE html><html><head><style>
    body{font-family:-apple-system,sans-serif;padding:20px;color:#e0e0e0;background:#1e1e1e}
    h1{font-size:1.3rem}table{width:100%;border-collapse:collapse;margin:10px 0}
    td,th{padding:6px 10px;text-align:left;border-bottom:1px solid #333}
    .metric{display:inline-block;margin:0 20px 10px 0;text-align:center}
    .metric-value{font-size:1.5rem;font-weight:bold}
    .metric-label{font-size:0.75rem;color:#888}
  </style></head><body>
    <h1>Stockyard — ${status.status}</h1>
    <p>Uptime: ${status.uptime} · Version: ${status.version || 'dev'} · ${status.go_version}</p>
    <div>
      <div class="metric"><div class="metric-value">${status.total_requests?.toLocaleString() || 0}</div><div class="metric-label">Requests</div></div>
      <div class="metric"><div class="metric-value">${status.avg_latency_ms?.toFixed(2) || '0.00'} ms</div><div class="metric-label">Avg Latency</div></div>
      <div class="metric"><div class="metric-value">${((status.error_rate || 0) * 100).toFixed(2)}%</div><div class="metric-label">Error Rate</div></div>
      <div class="metric"><div class="metric-value">${status.memory?.alloc_mb?.toFixed(1) || '?'} MB</div><div class="metric-label">Memory</div></div>
    </div>
    <h2>Components</h2>
    <table><tr><th>Component</th><th>Status</th><th>Detail</th></tr>${comps}</table>
  </body></html>`;
}

function renderTracesHTML(data: any): string {
  const rows = (data.traces || []).map((t: any) =>
    `<tr><td>${t.model}</td><td>${t.provider}</td><td>${t.latency_ms}ms</td><td>${t.tokens_total || '?'}</td><td>$${t.cost || '0.00'}</td><td>${t.status}</td></tr>`
  ).join('');
  return `<!DOCTYPE html><html><head><style>
    body{font-family:-apple-system,sans-serif;padding:20px;color:#e0e0e0;background:#1e1e1e}
    h1{font-size:1.3rem}table{width:100%;border-collapse:collapse}
    td,th{padding:6px 10px;text-align:left;border-bottom:1px solid #333}
  </style></head><body>
    <h1>Recent Traces</h1>
    <table><tr><th>Model</th><th>Provider</th><th>Latency</th><th>Tokens</th><th>Cost</th><th>Status</th></tr>${rows}</table>
  </body></html>`;
}
