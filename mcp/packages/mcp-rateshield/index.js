#!/usr/bin/env node
/**
 * @stockyard/mcp-rateshield — Bulletproof your LLM rate limits
 * 
 * MCP server for Stockyard RateShield.
 * Rate limiting and request queuing for LLM APIs via MCP.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-rateshield
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-rateshield"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("rateshield");
server.start();
