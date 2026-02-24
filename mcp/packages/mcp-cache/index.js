#!/usr/bin/env node
/**
 * @stockyard/mcp-cache — Stop paying twice for the same LLM response
 * 
 * MCP server for Stockyard CacheLayer.
 * LLM response caching with configurable TTL via MCP.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-cache
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-cache"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("llmcache");
server.start();
