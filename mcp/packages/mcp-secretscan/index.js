#!/usr/bin/env node
/**
 * @stockyard/mcp-secretscan — Catch API keys leaking through LLM calls
 * 
 * MCP server for Stockyard SecretScan.
 * Detect and redact API keys, AWS credentials, tokens, and secrets in LLM requests and responses.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-secretscan
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-secretscan"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("secretscan");
server.start();
