#!/usr/bin/env node
/**
 * @stockyard/mcp-sessionstore — Managed conversation sessions
 * 
 * MCP server for Stockyard SessionStore.
 * Create/resume/list/delete sessions. Full history. Metadata. Concurrent limits.
 * 
 * Usage: npx @stockyard/mcp-sessionstore
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("sessionstore");
server.start();
