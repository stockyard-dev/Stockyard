#!/usr/bin/env node
/**
 * @stockyard/mcp-geoprice — Purchasing power pricing by region
 * 
 * MCP server for Stockyard GeoPrice.
 * PPP-adjusted pricing. Anti-VPN. Revenue by region dashboard.
 * 
 * Usage: npx @stockyard/mcp-geoprice
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("geoprice");
server.start();
