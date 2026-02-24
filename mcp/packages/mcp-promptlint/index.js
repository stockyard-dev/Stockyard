#!/usr/bin/env node
/**
 * @stockyard/mcp-promptlint — Catch prompt anti-patterns before they cost you money
 * 
 * MCP server for Stockyard PromptLint.
 * Static analysis for prompts: detect redundancy, injection patterns, excessive length. Score and suggest improvements.
 * 
 * Usage: npx @stockyard/mcp-promptlint
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("promptlint");
server.start();
