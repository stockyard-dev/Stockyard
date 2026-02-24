#!/usr/bin/env node
/**
 * @stockyard/mcp-chatmem — Persistent conversation memory without token bloat
 * 
 * MCP server for Stockyard ChatMem.
 * Conversation memory middleware.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-chatmem
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-chatmem"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("chatmem");
server.start();
