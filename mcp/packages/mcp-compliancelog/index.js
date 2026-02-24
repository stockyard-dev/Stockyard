#!/usr/bin/env node
/**
 * @stockyard/mcp-compliancelog — Immutable audit trail for every LLM call
 * 
 * MCP server for Stockyard ComplianceLog.
 * Tamper-proof audit logging for LLM interactions.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-compliancelog
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-compliancelog"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("compliancelog");
server.start();
