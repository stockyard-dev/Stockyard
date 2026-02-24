#!/usr/bin/env node
/**
 * @stockyard/mcp-streamthrottle — Control streaming speed for better UX
 * 
 * MCP server for Stockyard StreamThrottle.
 * Max tokens/sec. Buffer fast streams. Per endpoint/model/client.
 * 
 * Usage: npx @stockyard/mcp-streamthrottle
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("streamthrottle");
server.start();
