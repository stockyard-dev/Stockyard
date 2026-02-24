#!/usr/bin/env node
/**
 * @stockyard/mcp-consentgate — User consent management for AI interactions
 * 
 * MCP server for Stockyard ConsentGate.
 * Check consent per user. Block non-consented. Track timestamps. Support withdrawal.
 * 
 * Usage: npx @stockyard/mcp-consentgate
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("consentgate");
server.start();
