#!/usr/bin/env node
/**
 * @stockyard/mcp-promptchain — Composable prompt blocks
 * 
 * MCP server for Stockyard PromptChain.
 * Define reusable blocks. Compose: [tone.helpful, format.json, domain.ecommerce]. Auto-update.
 * 
 * Usage: npx @stockyard/mcp-promptchain
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("promptchain");
server.start();
