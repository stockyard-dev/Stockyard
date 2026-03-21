/**
 * Stockyard Desktop — Tauri main entry point.
 *
 * Features:
 * - System tray with live cost ticker
 * - Native window opening Stockyard console
 * - OS notifications for alerts
 * - Auto-start Stockyard binary
 * - Settings management
 */

interface Config {
  stockyardUrl: string;
  adminKey: string;
  notifications: {
    costAlerts: boolean;
    providerFailures: boolean;
    guardrailViolations: boolean;
  };
  autoStart: boolean;
}

const DEFAULT_CONFIG: Config = {
  stockyardUrl: "http://localhost:4200",
  adminKey: "",
  notifications: {
    costAlerts: true,
    providerFailures: true,
    guardrailViolations: true,
  },
  autoStart: true,
};

// Load config from disk.
async function loadConfig(): Promise<Config> {
  try {
    // In a real Tauri app, this would use @tauri-apps/api/fs
    const stored = localStorage.getItem("stockyard-config");
    if (stored) return { ...DEFAULT_CONFIG, ...JSON.parse(stored) };
  } catch {}
  return DEFAULT_CONFIG;
}

// Save config to disk.
async function saveConfig(config: Config): Promise<void> {
  localStorage.setItem("stockyard-config", JSON.stringify(config));
}

// Fetch live cost from Stockyard API.
async function fetchLiveCost(config: Config): Promise<string> {
  try {
    const resp = await fetch(`${config.stockyardUrl}/api/observe/costs?period=today`, {
      headers: { "X-Admin-Key": config.adminKey },
    });
    const data = await resp.json();
    const total = data.total_cost_usd || 0;
    return `$${total.toFixed(2)}`;
  } catch {
    return "$-.--";
  }
}

// Connect to SSE for real-time events.
async function connectSSE(config: Config): Promise<void> {
  const es = new EventSource(`${config.stockyardUrl}/ui/events`);
  es.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data);
      handleEvent(data, config);
    } catch {}
  };
}

// Handle incoming SSE events.
function handleEvent(event: any, config: Config): void {
  const type = event.type;

  if (type === "cost_alert" && config.notifications.costAlerts) {
    showNotification("Cost Alert", `Spending threshold reached: $${event.amount}`);
  }
  if (type === "provider_failure" && config.notifications.providerFailures) {
    showNotification("Provider Failure", `${event.provider} is experiencing issues`);
  }
  if (type === "guardrail_violation" && config.notifications.guardrailViolations) {
    showNotification("Guardrail Violation", `Rule violated: ${event.rule}`);
  }
}

// Show OS notification.
function showNotification(title: string, body: string): void {
  // In a real Tauri app, this would use @tauri-apps/api/notification
  if ("Notification" in window && Notification.permission === "granted") {
    new Notification(`Stockyard: ${title}`, { body });
  }
}

// Initialize the desktop app.
async function init(): Promise<void> {
  const config = await loadConfig();

  // Update tray cost ticker every 30s.
  setInterval(async () => {
    const cost = await fetchLiveCost(config);
    document.getElementById("cost-display")!.textContent = cost;
  }, 30000);

  // Connect to SSE for real-time events.
  connectSSE(config);

  // Initial cost fetch.
  const cost = await fetchLiveCost(config);
  document.getElementById("cost-display")!.textContent = cost;

  console.log("Stockyard Desktop initialized", config.stockyardUrl);
}

init();
