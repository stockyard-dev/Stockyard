#!/usr/bin/env node
/**
 * @stockyard/mcp-jsonguard — LLM responses that always parse
 * 
 * MCP server for Stockyard StructuredShield.
 * JSON schema validation for LLM responses via MCP.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-jsonguard
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-jsonguard"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("jsonguard");
server.start();
