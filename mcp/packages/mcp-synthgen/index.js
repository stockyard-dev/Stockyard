#!/usr/bin/env node
/**
 * @stockyard/mcp-synthgen — Generate synthetic training data through your proxy
 * 
 * MCP server for Stockyard SynthGen.
 * Templates + seed examples → synthetic training data at scale. Quality-checked through EvalGate.
 * 
 * Usage: npx @stockyard/mcp-synthgen
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("synthgen");
server.start();
