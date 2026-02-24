#!/usr/bin/env node
/**
 * @stockyard/mcp-llmtap — Full-stack LLM analytics in one binary
 * 
 * MCP server for Stockyard LLMTap.
 * API analytics portal for LLM traffic.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-llmtap
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-llmtap"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("llmtap");
server.start();
