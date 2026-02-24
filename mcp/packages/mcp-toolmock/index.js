#!/usr/bin/env node
/**
 * @stockyard/mcp-toolmock — Fake tool responses for testing
 * 
 * MCP server for Stockyard ToolMock.
 * Canned responses by tool+args. Simulate errors, timeouts, partial results.
 * 
 * Usage: npx @stockyard/mcp-toolmock
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("toolmock");
server.start();
