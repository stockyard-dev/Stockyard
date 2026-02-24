#!/usr/bin/env node
/**
 * @stockyard/mcp-convofork — Branch conversations — try different paths
 * 
 * MCP server for Stockyard ConvoFork.
 * Fork at any message. Independent history per branch. Tree visualization.
 * 
 * Usage: npx @stockyard/mcp-convofork
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("convofork");
server.start();
