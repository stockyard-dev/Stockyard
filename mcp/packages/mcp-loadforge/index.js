#!/usr/bin/env node
/**
 * @stockyard/mcp-loadforge — Load test your LLM stack
 * 
 * MCP server for Stockyard LoadForge.
 * Define load profiles. Measure TTFT, TPS, p50/p95/p99, errors.
 * 
 * Usage: npx @stockyard/mcp-loadforge
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("loadforge");
server.start();
