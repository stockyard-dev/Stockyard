#!/usr/bin/env node
/**
 * @stockyard/mcp-diffprompt — Git-style diff for prompt changes
 * 
 * MCP server for Stockyard DiffPrompt.
 * Track system prompt changes. Hash-based detection. See which models had prompt modifications.
 * 
 * Usage: npx @stockyard/mcp-diffprompt
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("diffprompt");
server.start();
