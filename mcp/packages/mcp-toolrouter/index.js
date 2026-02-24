#!/usr/bin/env node
/**
 * @stockyard/mcp-toolrouter — Manage, version, and route LLM function calls
 * 
 * MCP server for Stockyard ToolRouter.
 * Versioned tool schemas. Route calls. Shadow-test. Usage analytics.
 * 
 * Usage: npx @stockyard/mcp-toolrouter
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("toolrouter");
server.start();
