#!/usr/bin/env node
/**
 * @stockyard/mcp-mockllm — Deterministic LLM responses for testing
 * 
 * MCP server for Stockyard MockLLM.
 * Mock LLM server with canned responses for CI/CD pipelines.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-mockllm
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-mockllm"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("mockllm");
server.start();
