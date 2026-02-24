#!/usr/bin/env node
/**
 * Stockyard MCP Server Factory
 * Creates a fully functional MCP server for any Stockyard product.
 * 
 * Usage: createMCPServer("costcap") — returns an MCPServer ready to .start()
 * 
 * What it does:
 * 1. Downloads the correct Stockyard binary for the current platform
 * 2. Writes a YAML config with the product's defaults + user env vars
 * 3. Starts the proxy as a background process
 * 4. Registers MCP tools that call the proxy's management API
 * 5. Handles the MCP protocol over stdio
 */

const { MCPServer } = require("./mcp-server");
const { ensureBinary, writeConfig, startProxy, checkHealth, apiCall } = require("./binary");
const { PRODUCTS } = require("./products");
const http = require("http");

const VERSION = "0.1.0";

/**
 * Create and configure an MCP server for a specific Stockyard product.
 * @param {string} productKey - Product key from PRODUCTS (e.g. "costcap")
 * @param {object} [overrides] - Optional config overrides
 * @returns {MCPServer}
 */
function createMCPServer(productKey, overrides = {}) {
  const product = PRODUCTS[productKey];
  if (!product) {
    throw new Error(`Unknown product: ${productKey}. Available: ${Object.keys(PRODUCTS).join(", ")}`);
  }

  const port = overrides.port || product.port;
  const server = new MCPServer({
    name: `stockyard-${productKey}`,
    version: VERSION,
    description: product.description,
  });

  let proxyProcess = null;
  let proxyReady = false;

  // ─── Helper: make HTTP request to proxy API ────────────────────────
  function proxyRequest(path, method = "GET", body = null) {
    return new Promise((resolve, reject) => {
      const opts = {
        hostname: "127.0.0.1",
        port,
        path,
        method,
        headers: { "Content-Type": "application/json" },
      };
      const req = http.request(opts, (res) => {
        let data = "";
        res.on("data", (chunk) => (data += chunk));
        res.on("end", () => {
          try { resolve(JSON.parse(data)); }
          catch { resolve({ status: res.statusCode, raw: data }); }
        });
      });
      req.on("error", (err) => reject(new Error(`Proxy not reachable: ${err.message}. Is ${product.displayName} running on port ${port}?`)));
      req.setTimeout(5000, () => { req.destroy(); reject(new Error("Request timeout")); });
      if (body) req.write(JSON.stringify(body));
      req.end();
    });
  }

  // ─── Register: setup_proxy tool ────────────────────────────────────
  server.tool(
    `${productKey}_setup`,
    `Download and start the ${product.displayName} proxy. Call this first if the proxy isn't running. Requires API keys in environment variables.`,
    {
      type: "object",
      properties: {
        openai_key: { type: "string", description: "OpenAI API key (or set OPENAI_API_KEY env var)" },
        anthropic_key: { type: "string", description: "Anthropic API key (optional)" },
        groq_key: { type: "string", description: "Groq API key (optional)" },
      },
    },
    async (args) => {
      try {
        // Check if already running
        const healthy = await checkHealth(port);
        if (healthy) {
          proxyReady = true;
          return { content: [{ type: "text", text: `✓ ${product.displayName} is already running on port ${port}` }] };
        }

        // Download binary
        await ensureBinary(product.binary, VERSION);

        // Build config with provided keys or env vars
        const config = JSON.parse(JSON.stringify(product.defaultConfig));
        config.port = port;

        if (config.providers?.openai) {
          config.providers.openai.api_key = args.openai_key || process.env.OPENAI_API_KEY || "${OPENAI_API_KEY}";
        }
        if (config.providers?.anthropic) {
          config.providers.anthropic.api_key = args.anthropic_key || process.env.ANTHROPIC_API_KEY || "${ANTHROPIC_API_KEY}";
        }
        if (config.providers?.groq) {
          config.providers.groq.api_key = args.groq_key || process.env.GROQ_API_KEY || "${GROQ_API_KEY}";
        }

        // Write config and start
        const configPath = writeConfig(productKey, config);
        proxyProcess = startProxy(product.binary, configPath);

        // Wait for proxy to be ready (max 10s)
        for (let i = 0; i < 20; i++) {
          await new Promise((r) => setTimeout(r, 500));
          const up = await checkHealth(port);
          if (up) {
            proxyReady = true;
            return {
              content: [{
                type: "text",
                text: `✓ ${product.displayName} started on port ${port}\n` +
                  `  Proxy URL: http://127.0.0.1:${port}/v1/chat/completions\n` +
                  `  Dashboard: http://127.0.0.1:${port}/ui\n` +
                  `  Configure your LLM client to use this as the base URL.`,
              }],
            };
          }
        }

        return { content: [{ type: "text", text: `⏳ ${product.displayName} started but not yet responding. Check logs.` }] };
      } catch (err) {
        return { content: [{ type: "text", text: `✗ Failed to start ${product.displayName}: ${err.message}` }], isError: true };
      }
    }
  );

  // ─── Register: product-specific tools ──────────────────────────────
  for (const toolDef of product.tools) {
    server.tool(
      toolDef.name,
      toolDef.description,
      toolDef.inputSchema,
      async (args) => {
        try {
          const method = toolDef.method || "GET";
          let path = toolDef.apiPath;

          // Append query params for GET requests
          if (method === "GET" && Object.keys(args).length > 0) {
            const params = new URLSearchParams(args).toString();
            path = `${path}?${params}`;
          }

          const result = await proxyRequest(path, method, method !== "GET" ? args : null);
          return {
            content: [{
              type: "text",
              text: typeof result === "object" ? JSON.stringify(result, null, 2) : String(result),
            }],
          };
        } catch (err) {
          return {
            content: [{
              type: "text",
              text: `Error calling ${toolDef.name}: ${err.message}\n\nHint: Make sure the proxy is running. Call ${productKey}_setup first.`,
            }],
            isError: true,
          };
        }
      }
    );
  }

  // ─── Register: configure_client helper ─────────────────────────────
  server.tool(
    `${productKey}_configure_client`,
    `Get instructions for configuring your LLM application to route through ${product.displayName}.`,
    {
      type: "object",
      properties: {
        client: {
          type: "string",
          description: "Your LLM client",
          enum: ["openai-python", "openai-node", "langchain", "cursor", "curl", "other"],
          default: "other",
        },
      },
    },
    async (args) => {
      const baseUrl = `http://127.0.0.1:${port}/v1`;
      const client = args.client || "other";
      let instructions = `# Configure your app to use ${product.displayName}\n\n`;

      switch (client) {
        case "openai-python":
          instructions += `\`\`\`python\nfrom openai import OpenAI\nclient = OpenAI(base_url="${baseUrl}", api_key="any-string")\n\`\`\``;
          break;
        case "openai-node":
          instructions += `\`\`\`javascript\nconst client = new OpenAI({ baseURL: "${baseUrl}", apiKey: "any-string" });\n\`\`\``;
          break;
        case "langchain":
          instructions += `\`\`\`python\nfrom langchain_openai import ChatOpenAI\nllm = ChatOpenAI(base_url="${baseUrl}", api_key="any-string")\n\`\`\``;
          break;
        case "cursor":
          instructions += `Add to .cursor/mcp.json:\n\`\`\`json\n{ "mcpServers": { "stockyard-${productKey}": { "command": "npx", "args": ["@stockyard/mcp-${productKey}"] } } }\n\`\`\``;
          break;
        case "curl":
          instructions += `\`\`\`bash\ncurl ${baseUrl}/chat/completions \\\n  -H "Content-Type: application/json" \\\n  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}'\n\`\`\``;
          break;
        default:
          instructions += `Set your OpenAI base URL to: ${baseUrl}\nSet API key to any non-empty string (the proxy handles auth).\nDashboard: http://127.0.0.1:${port}/ui`;
      }

      return { content: [{ type: "text", text: instructions }] };
    }
  );

  // Cleanup on exit
  process.on("exit", () => {
    if (proxyProcess) {
      try { proxyProcess.kill(); } catch {}
    }
  });
  process.on("SIGINT", () => process.exit(0));
  process.on("SIGTERM", () => process.exit(0));

  return server;
}

module.exports = { createMCPServer, VERSION };
