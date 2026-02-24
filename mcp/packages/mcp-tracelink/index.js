#!/usr/bin/env node
/**
 * @stockyard/mcp-tracelink — Distributed tracing for LLM chains
 * 
 * MCP server for Stockyard TraceLink.
 * Link related LLM calls into trace trees.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-tracelink
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-tracelink"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("tracelink");
server.start();
