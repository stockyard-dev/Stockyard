#!/usr/bin/env node
/**
 * @stockyard/mcp-regionroute — Data residency routing for GDPR compliance
 * 
 * MCP server for Stockyard RegionRoute.
 * Route requests to region-specific endpoints. Keep EU data in EU. Geographic compliance made easy.
 * 
 * Usage: npx @stockyard/mcp-regionroute
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("regionroute");
server.start();
