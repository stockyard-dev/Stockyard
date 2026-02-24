#!/usr/bin/env node
/**
 * @stockyard/mcp-promptslim — Compress prompts by 40-70% without losing meaning
 * 
 * MCP server for Stockyard PromptSlim.
 * Remove redundant whitespace, filler words, articles. Configurable aggressiveness. See before/after token savings.
 * 
 * Usage: npx @stockyard/mcp-promptslim
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("promptslim");
server.start();
