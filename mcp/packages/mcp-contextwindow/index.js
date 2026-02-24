#!/usr/bin/env node
/**
 * @stockyard/mcp-contextwindow — Visual context window debugger
 * 
 * MCP server for Stockyard ContextWindow.
 * Visualize token allocation by message role. See what's eating your context window. Optimization recommendations.
 * 
 * Usage: npx @stockyard/mcp-contextwindow
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("contextwindow");
server.start();
