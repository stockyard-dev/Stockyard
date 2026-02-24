#!/usr/bin/env node
/**
 * @stockyard/mcp-localsync — Seamlessly blend local and cloud models
 * 
 * MCP server for Stockyard LocalSync.
 * Route to Ollama locally when available. Auto-failover to cloud when local is down. Track cost savings.
 * 
 * Usage: npx @stockyard/mcp-localsync
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("localsync");
server.start();
