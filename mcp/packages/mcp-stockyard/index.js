#!/usr/bin/env node
/**
 * @stockyard/mcp-stockyard — The complete LLM infrastructure suite
 * 
 * MCP server for Stockyard Stockyard.
 * 6 apps, 58 modules in one binary.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-stockyard
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-stockyard"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("stockyard");
server.start();
