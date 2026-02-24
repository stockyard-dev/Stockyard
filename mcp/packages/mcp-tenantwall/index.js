#!/usr/bin/env node
/**
 * @stockyard/mcp-tenantwall — Per-tenant isolation for multi-tenant LLM apps
 * 
 * MCP server for Stockyard TenantWall.
 * Tenant isolation middleware.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-tenantwall
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-tenantwall"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("tenantwall");
server.start();
