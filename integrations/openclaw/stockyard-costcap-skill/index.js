#!/usr/bin/env node
/**
 * Stockyard Cost Caps — OpenClaw Skill
 * Real-time spend tracking and budget caps. Never get a surprise LLM bill again.
 * 
 * Free tier: $10 daily cap
 * Paid: custom caps, per-model budgets, historical analytics
 */

const { ensureBinary, startProxy, apiCall, checkHealth, waitForProxy } = require("../shared/core");
const path = require("path");
const fs = require("fs");

const PORT = 4100;
const PRODUCT = "costcap";
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
    case "spend_query": {
      const data = await api(PORT, "/api/spend?project=default"); return "Today: $" + (data.today||0).toFixed(4) + " | Month: $" + (data.month||0).toFixed(4);
    }

    case "budget_set": {
      const amount = parseFloat(args.amount || "10"); await api(PORT, "/api/budget", "POST", {project:"default",daily:amount}); return "Daily budget set to $" + amount.toFixed(2);
    }
    default:
      return "Unknown tool: " + toolName;
  }
}

// OpenClaw skill interface
module.exports = {
  name: "stockyard-costcap-skill",
  displayName: "Stockyard Cost Caps",
  description: "Real-time spend tracking and budget caps. Never get a surprise LLM bill again.",
  
  async onInstall() {
    console.log("[Stockyard Cost Caps] Installing...");
    await ensureBinary(PRODUCT);
    console.log("[Stockyard Cost Caps] ✓ Ready");
  },
  
  async onUninstall() {
    if (proxyProcess) {
      try { proxyProcess.kill(); } catch {}
    }
  },
  
  tools: {
    spend_query: async (args) => handleTool("spend_query", args || {}),
    budget_set: async (args) => handleTool("budget_set", args || {}),
  },
};

// Cleanup
process.on("exit", () => { if (proxyProcess) try { proxyProcess.kill(); } catch {} });
