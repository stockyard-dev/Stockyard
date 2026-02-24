#!/usr/bin/env node
/**
 * @stockyard/mcp-guardrail — Keep your LLM on-script
 * 
 * MCP server for Stockyard GuardRail.
 * Topic fencing middleware. Define allowed/denied topics. Block off-topic responses with custom fallback messages.
 * 
 * Usage: npx @stockyard/mcp-guardrail
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("guardrail");
server.start();
