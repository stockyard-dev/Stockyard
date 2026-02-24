#!/usr/bin/env node
/**
 * @stockyard/mcp-retrypilot — Intelligent retries that actually work
 * 
 * MCP server for Stockyard RetryPilot.
 * Smart retry engine with exponential backoff, circuit breakers, deadline awareness, and automatic model downgrade on failures.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-retrypilot
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-retrypilot"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("retrypilot");
server.start();
