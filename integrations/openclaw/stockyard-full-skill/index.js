#!/usr/bin/env node
/**
 * Stockyard Suite — OpenClaw Skill
 * Complete LLM infrastructure in one install. Cost caps, caching, PII redaction, smart routing, analytics, and more.
 * 
 * Free tier: cache + basic cost cap + basic retry
 * Paid: full 20-tool suite
 */

const { ensureBinary, startProxy, apiCall, checkHealth, waitForProxy } = require("../shared/core");
const path = require("path");
const fs = require("fs");

const PORT = 4000;
const PRODUCT = "stockyard";
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
    case "stockyard_status": {
      const data = await api(PORT, "/api/status"); return "Stockyard: " + (data.status || "running") + " | Features: " + (data.enabled_features||[]).join(", ");
    }

    case "stockyard_spend": {
      const data = await api(PORT, "/api/spend?project=default"); return "Today: $" + (data.today||0).toFixed(4);
    }

    case "stockyard_cache": {
      const data = await api(PORT, "/api/cache/stats"); return "Hit rate: " + ((data.hits/(data.hits+data.misses)*100)||0).toFixed(1) + "%";
    }
    default:
      return "Unknown tool: " + toolName;
  }
}

// OpenClaw skill interface
module.exports = {
  name: "stockyard-full-skill",
  displayName: "Stockyard Suite",
  description: "Complete LLM infrastructure in one install. Cost caps, caching, PII redaction, smart routing, analytics, and more.",
  
  async onInstall() {
    console.log("[Stockyard Suite] Installing...");
    await ensureBinary(PRODUCT);
    console.log("[Stockyard Suite] ✓ Ready");
  },
  
  async onUninstall() {
    if (proxyProcess) {
      try { proxyProcess.kill(); } catch {}
    }
  },
  
  tools: {
    stockyard_status: async (args) => handleTool("stockyard_status", args || {}),
    stockyard_spend: async (args) => handleTool("stockyard_spend", args || {}),
    stockyard_cache: async (args) => handleTool("stockyard_cache", args || {}),
  },
};

// Cleanup
process.on("exit", () => { if (proxyProcess) try { proxyProcess.kill(); } catch {} });
