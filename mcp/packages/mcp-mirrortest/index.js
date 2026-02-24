#!/usr/bin/env node
/**
 * @stockyard/mcp-mirrortest — Shadow test new models against production traffic
 * 
 * MCP server for Stockyard MirrorTest.
 * Send production traffic to a shadow model. Compare quality, latency, cost. Zero user impact.
 * 
 * Usage: npx @stockyard/mcp-mirrortest
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("mirrortest");
server.start();
