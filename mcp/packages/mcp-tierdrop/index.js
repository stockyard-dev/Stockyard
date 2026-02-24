#!/usr/bin/env node
/**
 * @stockyard/mcp-tierdrop — Auto-downgrade models when burning cash
 * 
 * MCP server for Stockyard TierDrop.
 * Gracefully degrade from GPT-4 to GPT-3.5 when approaching budget limits. Cost-aware model selection.
 * 
 * Usage: npx @stockyard/mcp-tierdrop
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("tierdrop");
server.start();
