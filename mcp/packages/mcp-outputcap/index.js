#!/usr/bin/env node
/**
 * @stockyard/mcp-outputcap — Stop paying for responses you don't need
 * 
 * MCP server for Stockyard OutputCap.
 * Cap output length at natural sentence boundaries. No more 500-token essays when you asked for one word.
 * 
 * Usage: npx @stockyard/mcp-outputcap
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("outputcap");
server.start();
