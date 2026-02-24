#!/usr/bin/env node
/**
 * @stockyard/mcp-clidash — Terminal dashboard — htop for your LLM stack
 * 
 * MCP server for Stockyard CliDash.
 * Real-time TUI: req/sec, models, cache, spend, errors. SSH-accessible.
 * 
 * Usage: npx @stockyard/mcp-clidash
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("clidash");
server.start();
