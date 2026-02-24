#!/usr/bin/env node
/**
 * @stockyard/mcp-datamap — GDPR Article 30 data flow mapping
 * 
 * MCP server for Stockyard DataMap.
 * Auto-classify data. Map flows: source→proxy→provider→storage. Generate GDPR records.
 * 
 * Usage: npx @stockyard/mcp-datamap
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("datamap");
server.start();
