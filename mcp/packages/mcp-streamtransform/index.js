#!/usr/bin/env node
/**
 * @stockyard/mcp-streamtransform — Transform streaming responses mid-stream
 * 
 * MCP server for Stockyard StreamTransform.
 * Pipeline on chunks: strip markdown, redact PII, translate. Minimal latency.
 * 
 * Usage: npx @stockyard/mcp-streamtransform
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("streamtransform");
server.start();
