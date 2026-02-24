#!/usr/bin/env node
/**
 * @stockyard/mcp-costmap — Multi-dimensional cost attribution
 * 
 * MCP server for Stockyard CostMap.
 * Tag requests with dimensions. Drill-down: by feature, user, prompt.
 * 
 * Usage: npx @stockyard/mcp-costmap
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("costmap");
server.start();
