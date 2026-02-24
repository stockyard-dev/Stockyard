#!/usr/bin/env node
/**
 * @stockyard/mcp-authgate — API key management for YOUR users
 * 
 * MCP server for Stockyard AuthGate.
 * Issue/revoke keys to your customers. Per-key limits and usage tracking.
 * 
 * Usage: npx @stockyard/mcp-authgate
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("authgate");
server.start();
