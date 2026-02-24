#!/usr/bin/env node
/**
 * @stockyard/mcp-multicall — Ask multiple models, pick the best answer
 * 
 * MCP server for Stockyard MultiCall.
 * Send the same prompt to multiple LLMs simultaneously.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-multicall
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-multicall"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("multicall");
server.start();
