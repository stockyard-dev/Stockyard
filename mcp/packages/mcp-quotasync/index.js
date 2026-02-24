#!/usr/bin/env node
/**
 * @stockyard/mcp-quotasync — Track provider rate limits in real-time
 * 
 * MCP server for Stockyard QuotaSync.
 * Parse rate limit headers. Track per model/endpoint. Alert near limits.
 * 
 * Usage: npx @stockyard/mcp-quotasync
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("quotasync");
server.start();
