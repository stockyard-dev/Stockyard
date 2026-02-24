/**
 * Stockyard MCP Server — Lightweight MCP Protocol Implementation
 * Implements Model Context Protocol over stdio (JSON-RPC 2.0).
 * Zero external dependencies.
 */

class MCPServer {
  constructor({ name, version, description }) {
    this.name = name;
    this.version = version;
    this.description = description;
    this.tools = new Map();
    this._buffer = "";
  }

  /**
   * Register a tool.
   * @param {string} name - Tool name
   * @param {string} description - Human-readable description
   * @param {object} inputSchema - JSON Schema for parameters
   * @param {function} handler - async (args) => { content: [...] }
   */
  tool(name, description, inputSchema, handler) {
    this.tools.set(name, { name, description, inputSchema, handler });
  }

  /**
   * Start the server, reading from stdin and writing to stdout.
   */
  start() {
    process.stdin.setEncoding("utf-8");
    process.stdin.on("data", (chunk) => this._onData(chunk));
    process.stdin.on("end", () => process.exit(0));
    // Prevent unhandled rejection crashes
    process.on("unhandledRejection", (err) => {
      console.error(`[mcp] Unhandled rejection: ${err.message}`);
    });
  }

  _onData(chunk) {
    this._buffer += chunk;
    // MCP messages are newline-delimited JSON
    let newlineIdx;
    while ((newlineIdx = this._buffer.indexOf("\n")) !== -1) {
      const line = this._buffer.slice(0, newlineIdx).trim();
      this._buffer = this._buffer.slice(newlineIdx + 1);
      if (line.length > 0) {
        this._handleLine(line);
      }
    }
  }

  async _handleLine(line) {
    let msg;
    try {
      msg = JSON.parse(line);
    } catch {
      this._send({ jsonrpc: "2.0", error: { code: -32700, message: "Parse error" }, id: null });
      return;
    }

    const { method, params, id } = msg;

    try {
      let result;

      switch (method) {
        case "initialize":
          result = {
            protocolVersion: "2024-11-05",
            capabilities: {
              tools: { listChanged: false },
            },
            serverInfo: {
              name: this.name,
              version: this.version,
            },
          };
          break;

        case "notifications/initialized":
          // No response needed for notifications
          return;

        case "tools/list":
          result = {
            tools: Array.from(this.tools.values()).map((t) => ({
              name: t.name,
              description: t.description,
              inputSchema: t.inputSchema,
            })),
          };
          break;

        case "tools/call": {
          const toolName = params?.name;
          const toolArgs = params?.arguments || {};
          const tool = this.tools.get(toolName);
          if (!tool) {
            result = {
              content: [{ type: "text", text: `Unknown tool: ${toolName}` }],
              isError: true,
            };
          } else {
            try {
              result = await tool.handler(toolArgs);
            } catch (err) {
              result = {
                content: [{ type: "text", text: `Error: ${err.message}` }],
                isError: true,
              };
            }
          }
          break;
        }

        case "ping":
          result = {};
          break;

        default:
          if (id !== undefined) {
            this._send({ jsonrpc: "2.0", error: { code: -32601, message: `Unknown method: ${method}` }, id });
          }
          return;
      }

      if (id !== undefined) {
        this._send({ jsonrpc: "2.0", result, id });
      }
    } catch (err) {
      if (id !== undefined) {
        this._send({ jsonrpc: "2.0", error: { code: -32603, message: err.message }, id });
      }
    }
  }

  _send(msg) {
    process.stdout.write(JSON.stringify(msg) + "\n");
  }
}

module.exports = { MCPServer };
