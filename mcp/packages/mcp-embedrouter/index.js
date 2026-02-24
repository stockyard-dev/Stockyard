#!/usr/bin/env node
/**
 * @stockyard/mcp-embedrouter — Smart routing for embedding requests
 * 
 * MCP server for Stockyard EmbedRouter.
 * Batch over 50ms window. Deduplicate. Route by content type.
 * 
 * Usage: npx @stockyard/mcp-embedrouter
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("embedrouter");
server.start();
