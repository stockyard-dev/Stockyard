#!/usr/bin/env node
/**
 * @stockyard/mcp-tokenauction — Dynamic pricing based on demand
 * 
 * MCP server for Stockyard TokenAuction.
 * Monitor costs, queue, errors. Time-of-day pricing. Surge pricing.
 * 
 * Usage: npx @stockyard/mcp-tokenauction
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("tokenauction");
server.start();
