#!/usr/bin/env node
/**
 * @stockyard/mcp-promptreplay — Every LLM call, logged and replayable
 * 
 * MCP server for Stockyard PromptReplay.
 * Full request/response logging for LLM APIs.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-promptreplay
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-promptreplay"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("promptreplay");
server.start();
