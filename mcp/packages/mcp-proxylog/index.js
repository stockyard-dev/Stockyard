#!/usr/bin/env node
/**
 * @stockyard/mcp-proxylog — Structured logging for every proxy decision
 * 
 * MCP server for Stockyard ProxyLog.
 * Each middleware emits decision log. Per-request trace. X-Proxy-Trace header.
 * 
 * Usage: npx @stockyard/mcp-proxylog
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("proxylog");
server.start();
