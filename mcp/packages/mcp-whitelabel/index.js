#!/usr/bin/env node
/**
 * @stockyard/mcp-whitelabel — Your brand on Stockyard's engine
 * 
 * MCP server for Stockyard WhiteLabel.
 * Custom branding for resellers. Logo, colors, domain. Sell LLM infrastructure under your own brand.
 * 
 * Usage: npx @stockyard/mcp-whitelabel
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("whitelabel");
server.start();
