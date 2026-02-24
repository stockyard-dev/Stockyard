#!/usr/bin/env node
/**
 * @stockyard/mcp-partialcache — Cache reusable prompt prefixes
 * 
 * MCP server for Stockyard PartialCache.
 * Detect static system prompt prefix. Use native prefix caching where supported.
 * 
 * Usage: npx @stockyard/mcp-partialcache
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("partialcache");
server.start();
