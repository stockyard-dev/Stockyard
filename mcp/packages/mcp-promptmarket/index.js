#!/usr/bin/env node
/**
 * @stockyard/mcp-promptmarket — Community prompt library
 * 
 * MCP server for Stockyard PromptMarket.
 * Publish, browse, rate, fork prompts. Track which community prompts you use.
 * 
 * Usage: npx @stockyard/mcp-promptmarket
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("promptmarket");
server.start();
