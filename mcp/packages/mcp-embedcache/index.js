#!/usr/bin/env node
/**
 * @stockyard/mcp-embedcache — Never compute the same embedding twice
 * 
 * MCP server for Stockyard EmbedCache.
 * Embedding response caching for /v1/embeddings.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-embedcache
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-embedcache"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("embedcache");
server.start();
