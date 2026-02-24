#!/usr/bin/env node
/**
 * @stockyard/mcp-visionproxy — Proxy magic for vision/image APIs
 * 
 * MCP server for Stockyard VisionProxy.
 * Caching, cost tracking, and failover for GPT-4V, Claude vision.
 * 
 * Usage: npx @stockyard/mcp-visionproxy
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("visionproxy");
server.start();
