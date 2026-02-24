#!/usr/bin/env node
/**
 * @stockyard/mcp-clustermode — Run multiple instances with shared state
 * 
 * MCP server for Stockyard ClusterMode.
 * Multi-instance coordination. Leader-follower with shared cache. Scale beyond single-instance SQLite.
 * 
 * Usage: npx @stockyard/mcp-clustermode
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("clustermode");
server.start();
