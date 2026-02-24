#!/usr/bin/env node
/**
 * @stockyard/mcp-tokentrim — Never hit a context limit again
 * 
 * MCP server for Stockyard TokenTrim.
 * Automatic context window management.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-tokentrim
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-tokentrim"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("tokentrim");
server.start();
