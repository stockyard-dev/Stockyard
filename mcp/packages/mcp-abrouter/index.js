#!/usr/bin/env node
/**
 * @stockyard/mcp-abrouter — A/B test any LLM variable with statistical rigor
 * 
 * MCP server for Stockyard ABRouter.
 * Run experiments across models, prompts, temperatures. Weighted traffic splits with automatic significance testing.
 * 
 * Usage: npx @stockyard/mcp-abrouter
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("abrouter");
server.start();
