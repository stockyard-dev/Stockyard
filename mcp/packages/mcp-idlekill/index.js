#!/usr/bin/env node
/**
 * @stockyard/mcp-idlekill — Kill runaway LLM requests before they drain your wallet
 * 
 * MCP server for Stockyard IdleKill.
 * Request watchdog middleware.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-idlekill
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-idlekill"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("idlekill");
server.start();
