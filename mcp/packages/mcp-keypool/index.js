#!/usr/bin/env node
/**
 * @stockyard/mcp-keypool — Pool your API keys, multiply your limits
 * 
 * MCP server for Stockyard KeyPool.
 * API key pooling and rotation for LLM providers.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-keypool
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-keypool"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("keypool");
server.start();
