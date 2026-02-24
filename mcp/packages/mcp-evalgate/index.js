#!/usr/bin/env node
/**
 * @stockyard/mcp-evalgate — Only ship quality LLM responses
 * 
 * MCP server for Stockyard EvalGate.
 * Response quality scoring and auto-retry.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-evalgate
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-evalgate"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("evalgate");
server.start();
