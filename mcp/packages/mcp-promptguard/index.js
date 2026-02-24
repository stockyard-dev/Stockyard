#!/usr/bin/env node
/**
 * @stockyard/mcp-promptguard — PII never hits the LLM
 * 
 * MCP server for Stockyard PromptGuard.
 * PII redaction and prompt injection detection for LLM APIs.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-promptguard
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-promptguard"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("promptguard");
server.start();
