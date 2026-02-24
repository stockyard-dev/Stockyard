#!/usr/bin/env node
/**
 * @stockyard/mcp-edgecache — CDN-like caching for LLM responses
 * 
 * MCP server for Stockyard EdgeCache.
 * Distribute cache across instances. Geographic hit rates.
 * 
 * Usage: npx @stockyard/mcp-edgecache
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("edgecache");
server.start();
