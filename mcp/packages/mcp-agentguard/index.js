#!/usr/bin/env node
/**
 * @stockyard/mcp-agentguard — Safety rails for autonomous AI agents
 * 
 * MCP server for Stockyard AgentGuard.
 * Per-session limits for AI agents: max calls, cost, duration. Kill runaway agent sessions before they drain your budget.
 * 
 * Usage: npx @stockyard/mcp-agentguard
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("agentguard");
server.start();
