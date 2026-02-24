#!/usr/bin/env node
/**
 * @stockyard/mcp-llmbench — Benchmark any model on YOUR workload
 * 
 * MCP server for Stockyard LLMBench.
 * Per-model performance tracking: latency, cost, tokens. Compare models on your actual traffic.
 * 
 * Usage: npx @stockyard/mcp-llmbench
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("llmbench");
server.start();
