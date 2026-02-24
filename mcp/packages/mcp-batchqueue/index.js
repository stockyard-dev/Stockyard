#!/usr/bin/env node
/**
 * @stockyard/mcp-batchqueue — Background jobs for LLM calls
 * 
 * MCP server for Stockyard BatchQueue.
 * Async job queue for LLM requests with priority levels, concurrency control, and retry.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-batchqueue
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-batchqueue"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("batchqueue");
server.start();
