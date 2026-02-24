#!/usr/bin/env node
/**
 * @stockyard/mcp-promptpad — Version control for your prompts
 * 
 * MCP server for Stockyard PromptPad.
 * Prompt template versioning and A/B testing.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-promptpad
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-promptpad"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("promptpad");
server.start();
