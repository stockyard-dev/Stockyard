#!/usr/bin/env node
/**
 * @stockyard/mcp-contextpack — RAG without the vector database
 * 
 * MCP server for Stockyard ContextPack.
 * Rule-based context injection from local files.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-contextpack
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-contextpack"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("contextpack");
server.start();
