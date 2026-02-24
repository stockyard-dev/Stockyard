#!/usr/bin/env node
/**
 * @stockyard/mcp-costpredict — Predict request cost BEFORE sending
 * 
 * MCP server for Stockyard CostPredict.
 * Count input tokens. Estimate output. Calculate cost. X-Estimated-Cost header.
 * 
 * Usage: npx @stockyard/mcp-costpredict
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("costpredict");
server.start();
