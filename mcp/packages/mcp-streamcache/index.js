#!/usr/bin/env node
/**
 * @stockyard/mcp-streamcache — Cache streaming responses with realistic timing
 * 
 * MCP server for Stockyard StreamCache.
 * Store original chunk timing. Replay cached SSE with original pacing.
 * 
 * Usage: npx @stockyard/mcp-streamcache
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("streamcache");
server.start();
