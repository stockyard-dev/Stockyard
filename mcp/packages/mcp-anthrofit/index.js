#!/usr/bin/env node
/**
 * @stockyard/mcp-anthrofit — Use Claude with OpenAI SDKs
 * 
 * MCP server for Stockyard AnthroFit.
 * Deep Anthropic compatibility layer.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-anthrofit
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-anthrofit"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("anthrofit");
server.start();
