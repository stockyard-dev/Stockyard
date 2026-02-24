#!/usr/bin/env node
/**
 * @stockyard/mcp-agentreplay — Record and replay agent sessions step-by-step
 * 
 * MCP server for Stockyard AgentReplay.
 * Step-by-step playback on TraceLink data. What-if mode. Export as test cases.
 * 
 * Usage: npx @stockyard/mcp-agentreplay
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("agentreplay");
server.start();
