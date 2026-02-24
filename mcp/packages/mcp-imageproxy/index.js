#!/usr/bin/env node
/**
 * @stockyard/mcp-imageproxy — Proxy magic for image generation APIs
 * 
 * MCP server for Stockyard ImageProxy.
 * Cost tracking, caching, and failover for DALL-E and other image generation APIs.
 * 
 * Usage: npx @stockyard/mcp-imageproxy
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("imageproxy");
server.start();
