#!/usr/bin/env node
/**
 * @stockyard/mcp-queuepriority — Priority queues — VIP users first
 * 
 * MCP server for Stockyard QueuePriority.
 * Priority levels per key/tenant. Reserved capacity. SLA tracking.
 * 
 * Usage: npx @stockyard/mcp-queuepriority
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("queuepriority");
server.start();
