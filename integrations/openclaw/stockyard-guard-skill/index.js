#!/usr/bin/env node
/**
 * Stockyard Guard — OpenClaw Skill
 * PII redaction and prompt injection detection. Your personal data never reaches the LLM provider.
 * 
 * Free tier: email/phone redaction
 * Paid: all PII patterns, custom rules, redact-restore, audit export
 */

const { ensureBinary, startProxy, apiCall, checkHealth, waitForProxy } = require("../shared/core");
const path = require("path");
const fs = require("fs");

const PORT = 4800;
const PRODUCT = "promptguard";
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
    case "guard_stats": {
      const data = await api(PORT, "/api/guard/stats"); return "Scanned: " + (data.total_scanned||0) + " | PII detected: " + (data.pii_detections||0) + " | Injections blocked: " + (data.injection_blocks||0);
    }

    case "guard_test": {
      const data = await api(PORT, "/api/guard/test", "POST", {text: args.text}); return data.pii_found ? "⚠️ PII found: " + data.patterns.join(", ") : "✓ No PII detected";
    }
    default:
      return "Unknown tool: " + toolName;
  }
}

// OpenClaw skill interface
module.exports = {
  name: "stockyard-guard-skill",
  displayName: "Stockyard Guard",
  description: "PII redaction and prompt injection detection. Your personal data never reaches the LLM provider.",
  
  async onInstall() {
    console.log("[Stockyard Guard] Installing...");
    await ensureBinary(PRODUCT);
    console.log("[Stockyard Guard] ✓ Ready");
  },
  
  async onUninstall() {
    if (proxyProcess) {
      try { proxyProcess.kill(); } catch {}
    }
  },
  
  tools: {
    guard_stats: async (args) => handleTool("guard_stats", args || {}),
    guard_test: async (args) => handleTool("guard_test", args || {}),
  },
};

// Cleanup
process.on("exit", () => { if (proxyProcess) try { proxyProcess.kill(); } catch {} });
