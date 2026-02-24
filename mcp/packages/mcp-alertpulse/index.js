#!/usr/bin/env node
/**
 * @stockyard/mcp-alertpulse — PagerDuty for your LLM stack
 * 
 * MCP server for Stockyard AlertPulse.
 * Configurable alerting for LLM infrastructure.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-alertpulse
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-alertpulse"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("alertpulse");
server.start();
