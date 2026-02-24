#!/usr/bin/env node
/**
 * @stockyard/mcp-modelswitch — Right model, right prompt, right price
 * 
 * MCP server for Stockyard ModelSwitch.
 * Smart model routing based on token count, prompt patterns, and headers.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-modelswitch
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-modelswitch"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("modelswitch");
server.start();
