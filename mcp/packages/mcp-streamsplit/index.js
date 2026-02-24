#!/usr/bin/env node
/**
 * @stockyard/mcp-streamsplit — Fork streaming responses to multiple destinations
 * 
 * MCP server for Stockyard StreamSplit.
 * Tee SSE chunks to logger, quality checker, webhook. Zero latency for primary.
 * 
 * Usage: npx @stockyard/mcp-streamsplit
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("streamsplit");
server.start();
