#!/usr/bin/env node
/**
 * @stockyard/mcp-ipfence — IP allowlisting for your LLM endpoints
 * 
 * MCP server for Stockyard IPFence.
 * IP-level access control for LLM proxy endpoints.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-ipfence
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-ipfence"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("ipfence");
server.start();
