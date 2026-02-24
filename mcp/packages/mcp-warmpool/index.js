#!/usr/bin/env node
/**
 * @stockyard/mcp-warmpool — Pre-warm model connections
 * 
 * MCP server for Stockyard WarmPool.
 * Persistent connections. Health checks. Keep-alive for Ollama.
 * 
 * Usage: npx @stockyard/mcp-warmpool
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("warmpool");
server.start();
