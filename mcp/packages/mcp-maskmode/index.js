#!/usr/bin/env node
/**
 * @stockyard/mcp-maskmode — Demo mode with realistic fake data
 * 
 * MCP server for Stockyard MaskMode.
 * Replace real PII in responses with realistic fakes. Consistent within session. Perfect for sales demos.
 * 
 * Usage: npx @stockyard/mcp-maskmode
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("maskmode");
server.start();
