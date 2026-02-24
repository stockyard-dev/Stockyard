#!/usr/bin/env node
/**
 * @stockyard/mcp-devproxy — Charles Proxy for LLM APIs
 * 
 * MCP server for Stockyard DevProxy.
 * Interactive debugging proxy. Log headers, bodies, latency for every request. Development inspection tool.
 * 
 * Usage: npx @stockyard/mcp-devproxy
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("devproxy");
server.start();
