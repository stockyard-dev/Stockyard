#!/usr/bin/env node
/**
 * @stockyard/mcp-chainforge — Multi-step LLM workflows as YAML pipelines
 * 
 * MCP server for Stockyard ChainForge.
 * Define extract→analyze→summarize→format pipelines. Conditional branching, parallel execution, cost tracking per pipeline.
 * 
 * Usage: npx @stockyard/mcp-chainforge
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("chainforge");
server.start();
