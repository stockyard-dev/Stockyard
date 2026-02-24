#!/usr/bin/env node
/**
 * @stockyard/mcp-tokenmarket — Dynamic budget reallocation across teams
 * 
 * MCP server for Stockyard TokenMarket.
 * Pool-based budgets. Teams request capacity. Auto-rebalance. Priority queuing for high-value requests.
 * 
 * Usage: npx @stockyard/mcp-tokenmarket
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("tokenmarket");
server.start();
