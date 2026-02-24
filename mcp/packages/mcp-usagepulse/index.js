#!/usr/bin/env node
/**
 * @stockyard/mcp-usagepulse — Know exactly where every token goes
 * 
 * MCP server for Stockyard UsagePulse.
 * Per-user and per-feature token metering.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-usagepulse
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-usagepulse"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("usagepulse");
server.start();
