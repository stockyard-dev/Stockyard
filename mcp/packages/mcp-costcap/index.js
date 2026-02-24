#!/usr/bin/env node
/**
 * @stockyard/mcp-costcap — Never get a surprise LLM bill again
 * 
 * MCP server for Stockyard CostCap.
 * LLM spending caps, budget tracking, and cost alerts via MCP.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-costcap
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-costcap"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("costcap");
server.start();
