#!/usr/bin/env node
/**
 * @stockyard/mcp-streamsnap — Capture and replay every LLM stream
 * 
 * MCP server for Stockyard StreamSnap.
 * SSE stream capture with zero latency overhead.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-streamsnap
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-streamsnap"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("streamsnap");
server.start();
