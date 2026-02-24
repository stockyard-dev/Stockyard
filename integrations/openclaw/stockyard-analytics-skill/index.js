#!/usr/bin/env node
/**
 * Stockyard Analytics — OpenClaw Skill
 * LLM usage analytics and reporting. Ask 'show my usage' for cost breakdowns and trends.
 * 
 * Free tier: 7-day lookback
 * Paid: 90-day history, scheduled reports, CSV export
 */

const { ensureBinary, startProxy, apiCall, checkHealth, waitForProxy } = require("../shared/core");
const path = require("path");
const fs = require("fs");

const PORT = 5300;
const PRODUCT = "llmtap";
let proxyProcess = null;

async function api(port, path, method, body) {
  return apiCall(port, path, method || "GET", body || null);
}

// Tool handler
async function handleTool(toolName, args) {
  // Ensure proxy is running
  if (!await checkHealth(PORT)) {
    const binPath = await ensureBinary(PRODUCT);
    const configDir = path.join(process.env.HOME || "/tmp", ".stockyard");
    fs.mkdirSync(configDir, { recursive: true });
    const configPath = path.join(configDir, PRODUCT + ".yaml");
    if (!fs.existsSync(configPath)) {
      fs.writeFileSync(configPath, `port: ${PORT}\ndata_dir: ${configDir}\nlog_level: info\nproduct: ${PRODUCT}\nproviders:\n  openai:\n    api_key: \${OPENAI_API_KEY}\n    base_url: https://api.openai.com/v1\n`);
    }
    proxyProcess = startProxy(binPath, configPath, PORT);
    await waitForProxy(PORT);
  }

  switch (toolName) {
    case "analytics_query": {
      const data = await api(PORT, "/api/analytics/overview?period=24h"); return "Requests: " + (data.total_requests||0) + " | Errors: " + (data.error_rate||0) + "% | Cost: $" + (data.total_cost||0).toFixed(4);
    }

    case "usage_report": {
      const data = await api(PORT, "/api/analytics/costs?period=7d&group_by=model"); return JSON.stringify(data, null, 2);
    }
    default:
      return "Unknown tool: " + toolName;
  }
}

// OpenClaw skill interface
module.exports = {
  name: "stockyard-analytics-skill",
  displayName: "Stockyard Analytics",
  description: "LLM usage analytics and reporting. Ask 'show my usage' for cost breakdowns and trends.",
  
  async onInstall() {
    console.log("[Stockyard Analytics] Installing...");
    await ensureBinary(PRODUCT);
    console.log("[Stockyard Analytics] ✓ Ready");
  },
  
  async onUninstall() {
    if (proxyProcess) {
      try { proxyProcess.kill(); } catch {}
    }
  },
  
  tools: {
    analytics_query: async (args) => handleTool("analytics_query", args || {}),
    usage_report: async (args) => handleTool("usage_report", args || {}),
  },
};

// Cleanup
process.on("exit", () => { if (proxyProcess) try { proxyProcess.kill(); } catch {} });
