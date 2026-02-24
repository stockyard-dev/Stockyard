/**
 * Stockyard Extension for SillyTavern
 * 
 * Routes all LLM calls through Stockyard proxy for cost tracking, caching,
 * rate limiting, and analytics. RP users making $200-500/mo in API calls
 * typically save 30-60% from response caching alone.
 * 
 * Install: Copy to SillyTavern/public/scripts/extensions/third-party/stockyard/
 */

const MODULE_NAME = "Stockyard";
const DEFAULT_PROXY_URL = "http://localhost:4000";

// Extension settings
const defaultSettings = {
    enabled: true,
    proxyUrl: DEFAULT_PROXY_URL,
    showCostInChat: true,
    showCacheHits: true,
    dailyBudget: 10.00,
    budgetWarningThreshold: 0.8,
};

let extensionSettings = { ...defaultSettings };

// Load settings from SillyTavern's extension storage
function loadSettings() {
    const stored = extension_settings?.stockyard;
    if (stored) {
        extensionSettings = { ...defaultSettings, ...stored };
    }
}

// Save settings
function saveSettings() {
    if (typeof extension_settings !== "undefined") {
        extension_settings.stockyard = extensionSettings;
        saveSettingsDebounced();
    }
}

// Settings UI HTML
function getSettingsHtml() {
    return `
    <div class="stockyard-settings">
        <div class="inline-drawer">
            <div class="inline-drawer-toggle inline-drawer-header">
                <b>🔧 Stockyard — Cost Control & Caching</b>
                <div class="inline-drawer-icon fa-solid fa-circle-chevron-down down"></div>
            </div>
            <div class="inline-drawer-content">
                <div class="flex-container">
                    <label for="stockyard_enabled">
                        <input id="stockyard_enabled" type="checkbox" ${extensionSettings.enabled ? "checked" : ""} />
                        Enable Stockyard Proxy
                    </label>
                </div>
                <label for="stockyard_proxy_url">Proxy URL</label>
                <input id="stockyard_proxy_url" type="text" value="${extensionSettings.proxyUrl}" placeholder="${DEFAULT_PROXY_URL}" />
                <label for="stockyard_daily_budget">Daily Budget ($)</label>
                <input id="stockyard_daily_budget" type="number" step="0.50" min="0" value="${extensionSettings.dailyBudget}" />
                <div class="flex-container">
                    <label for="stockyard_show_cost">
                        <input id="stockyard_show_cost" type="checkbox" ${extensionSettings.showCostInChat ? "checked" : ""} />
                        Show cost in chat
                    </label>
                </div>
                <div class="flex-container">
                    <label for="stockyard_show_cache">
                        <input id="stockyard_show_cache" type="checkbox" ${extensionSettings.showCacheHits ? "checked" : ""} />
                        Show cache hits
                    </label>
                </div>
                <div id="stockyard_status" class="flex-container" style="margin-top: 10px;">
                    <span>Status: checking...</span>
                </div>
                <div id="stockyard_spend" class="flex-container" style="margin-top: 5px;"></div>
            </div>
        </div>
    </div>`;
}

// Check proxy health
async function checkProxyHealth() {
    const statusEl = document.getElementById("stockyard_status");
    if (!statusEl) return;

    try {
        const r = await fetch(`${extensionSettings.proxyUrl}/health`, { signal: AbortSignal.timeout(3000) });
        if (r.ok) {
            statusEl.innerHTML = '<span style="color: #4CAF50;">✓ Connected</span>';
            updateSpendDisplay();
        } else {
            statusEl.innerHTML = '<span style="color: #f44336;">✗ Proxy error</span>';
        }
    } catch {
        statusEl.innerHTML = `<span style="color: #f44336;">✗ Not reachable — run: npx @stockyard/stockyard</span>`;
    }
}

// Update spend display
async function updateSpendDisplay() {
    const spendEl = document.getElementById("stockyard_spend");
    if (!spendEl) return;

    try {
        const r = await fetch(`${extensionSettings.proxyUrl}/api/spend?project=default`);
        if (!r.ok) return;
        const data = await r.json();
        const today = data.today || 0;
        const cap = extensionSettings.dailyBudget;
        const pct = cap > 0 ? (today / cap * 100).toFixed(0) : "∞";
        const color = (today / cap) > extensionSettings.budgetWarningThreshold ? "#f44336" : "#4CAF50";
        spendEl.innerHTML = `<span style="color: ${color};">💰 Today: $${today.toFixed(4)} / $${cap.toFixed(2)} (${pct}%)</span>`;
    } catch {
        // Silently fail
    }
}

// Intercept API calls to route through proxy
function setupProxyIntercept() {
    if (!extensionSettings.enabled) return;

    // Override the OpenAI base URL in SillyTavern's settings
    // This hooks into SillyTavern's API call mechanism
    const originalFetch = window.fetch;
    window.fetch = async function (...args) {
        let [url, options] = args;

        // Intercept OpenAI API calls
        if (typeof url === "string" && extensionSettings.enabled) {
            if (url.includes("api.openai.com/v1/") || url.includes("/v1/chat/completions")) {
                // Rewrite to proxy
                const proxyBase = extensionSettings.proxyUrl;
                const path = url.includes("/v1/") ? url.substring(url.indexOf("/v1/")) : "/v1/chat/completions";
                url = `${proxyBase}${path}`;

                // Add tracking headers
                if (options?.headers) {
                    const headers = new Headers(options.headers);
                    headers.set("X-Stockyard-User", "sillytavern");
                    headers.set("X-Stockyard-Feature", "roleplay");
                    options.headers = headers;
                }
            }
        }

        const response = await originalFetch.call(this, url, options);

        // Post-response: update spend display
        if (typeof url === "string" && url.includes(extensionSettings.proxyUrl)) {
            setTimeout(updateSpendDisplay, 500);
        }

        return response;
    };
}

// Initialize extension
jQuery(async () => {
    loadSettings();

    // Add settings UI
    const settingsHtml = getSettingsHtml();
    $("#extensions_settings2").append(settingsHtml);

    // Bind settings events
    $("#stockyard_enabled").on("change", function () {
        extensionSettings.enabled = this.checked;
        saveSettings();
    });
    $("#stockyard_proxy_url").on("input", function () {
        extensionSettings.proxyUrl = this.value || DEFAULT_PROXY_URL;
        saveSettings();
        checkProxyHealth();
    });
    $("#stockyard_daily_budget").on("input", function () {
        extensionSettings.dailyBudget = parseFloat(this.value) || 10.0;
        saveSettings();
        updateSpendDisplay();
    });
    $("#stockyard_show_cost").on("change", function () {
        extensionSettings.showCostInChat = this.checked;
        saveSettings();
    });
    $("#stockyard_show_cache").on("change", function () {
        extensionSettings.showCacheHits = this.checked;
        saveSettings();
    });

    // Setup proxy intercept
    setupProxyIntercept();

    // Check health
    checkProxyHealth();
    setInterval(updateSpendDisplay, 30000); // Update every 30s

    console.log(`[${MODULE_NAME}] Extension loaded`);
});
