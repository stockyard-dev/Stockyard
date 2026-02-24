#!/usr/bin/env node
/**
 * @stockyard/mcp-chaosllm — Chaos engineering for your LLM stack
 * 
 * MCP server for Stockyard ChaosLLM.
 * Inject realistic failures: 429s, timeouts, malformed JSON, truncated streams.
 * 
 * Usage: npx @stockyard/mcp-chaosllm
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("chaosllm");
server.start();
