#!/usr/bin/env node
/**
 * @stockyard/mcp-routefall — LLM calls that never fail
 * 
 * MCP server for Stockyard FallbackRouter.
 * Automatic failover between LLM providers via MCP.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-routefall
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-routefall"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("routefall");
server.start();
