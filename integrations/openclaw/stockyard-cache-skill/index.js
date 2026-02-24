#!/usr/bin/env node
/**
 * Stockyard Cache — OpenClaw Skill
 * Response caching for your LLM calls. Same question = instant cached answer, saves 30-50%.
 * 
 * Free tier: 1K cached entries
 * Paid: unlimited cache, semantic matching, TTL config
 */

const { ensureBinary, startProxy, apiCall, checkHealth, waitForProxy } = require("../shared/core");
const path = require("path");
const fs = require("fs");

const PORT = 4200;
const PRODUCT = "llmcache";
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
    case "cache_stats": {
      const data = await api(PORT, "/api/cache/stats"); const hits = data.hits||0; const total = hits + (data.misses||0); const rate = total > 0 ? (hits/total*100).toFixed(1) : "0"; return "Cache: " + rate + "% hit rate, saved $" + (data.estimated_savings||0).toFixed(2);
    }

    case "cache_clear": {
      await api(PORT, "/api/cache/flush", "POST"); return "Cache cleared.";
    }
    default:
      return "Unknown tool: " + toolName;
  }
}

// OpenClaw skill interface
module.exports = {
  name: "stockyard-cache-skill",
  displayName: "Stockyard Cache",
  description: "Response caching for your LLM calls. Same question = instant cached answer, saves 30-50%.",
  
  async onInstall() {
    console.log("[Stockyard Cache] Installing...");
    await ensureBinary(PRODUCT);
    console.log("[Stockyard Cache] ✓ Ready");
  },
  
  async onUninstall() {
    if (proxyProcess) {
      try { proxyProcess.kill(); } catch {}
    }
  },
  
  tools: {
    cache_stats: async (args) => handleTool("cache_stats", args || {}),
    cache_clear: async (args) => handleTool("cache_clear", args || {}),
  },
};

// Cleanup
process.on("exit", () => { if (proxyProcess) try { proxyProcess.kill(); } catch {} });
