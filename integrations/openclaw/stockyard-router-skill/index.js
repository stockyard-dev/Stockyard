#!/usr/bin/env node
/**
 * Stockyard Router — OpenClaw Skill
 * Smart model routing. Simple questions → cheap models. Complex reasoning → powerful models. Save 40-70%.
 * 
 * Free tier: 3 routing rules
 * Paid: unlimited rules, per-skill routing, A/B testing
 */

const { ensureBinary, startProxy, apiCall, checkHealth, waitForProxy } = require("../shared/core");
const path = require("path");
const fs = require("fs");

const PORT = 4900;
const PRODUCT = "modelswitch";
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
    case "routing_stats": {
      const data = await api(PORT, "/api/modelswitch/stats"); return "Routed: " + (data.total_routed||0) + " requests | Estimated savings: $" + (data.cost_saved||0).toFixed(2);
    }

    case "routing_test": {
      const data = await api(PORT, "/api/modelswitch/test", "POST", {text: args.text}); return "Would route to: " + (data.model || "default");
    }
    default:
      return "Unknown tool: " + toolName;
  }
}

// OpenClaw skill interface
module.exports = {
  name: "stockyard-router-skill",
  displayName: "Stockyard Router",
  description: "Smart model routing. Simple questions → cheap models. Complex reasoning → powerful models. Save 40-70%.",
  
  async onInstall() {
    console.log("[Stockyard Router] Installing...");
    await ensureBinary(PRODUCT);
    console.log("[Stockyard Router] ✓ Ready");
  },
  
  async onUninstall() {
    if (proxyProcess) {
      try { proxyProcess.kill(); } catch {}
    }
  },
  
  tools: {
    routing_stats: async (args) => handleTool("routing_stats", args || {}),
    routing_test: async (args) => handleTool("routing_test", args || {}),
  },
};

// Cleanup
process.on("exit", () => { if (proxyProcess) try { proxyProcess.kill(); } catch {} });
