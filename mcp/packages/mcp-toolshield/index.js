#!/usr/bin/env node
/**
 * @stockyard/mcp-toolshield — Validate and sandbox LLM tool calls
 * 
 * MCP server for Stockyard ToolShield.
 * Intercept tool_use. Validate args. Per-tool permissions and rate limits.
 * 
 * Usage: npx @stockyard/mcp-toolshield
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("toolshield");
server.start();
