#!/usr/bin/env node
/**
 * @stockyard/mcp-toxicfilter — Content moderation for LLM outputs
 * 
 * MCP server for Stockyard ToxicFilter.
 * Content moderation middleware for LLM responses.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-toxicfilter
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-toxicfilter"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("toxicfilter");
server.start();
