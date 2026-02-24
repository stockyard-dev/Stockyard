#!/usr/bin/env node
/**
 * @stockyard/mcp-spotprice — Real-time model pricing intelligence
 * 
 * MCP server for Stockyard SpotPrice.
 * Live pricing DB. Route to cheapest model meeting quality threshold.
 * 
 * Usage: npx @stockyard/mcp-spotprice
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("spotprice");
server.start();
