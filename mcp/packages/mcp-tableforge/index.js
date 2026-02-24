#!/usr/bin/env node
/**
 * @stockyard/mcp-tableforge — LLM-powered CSV/table generation with validation
 * 
 * MCP server for Stockyard TableForge.
 * Detect tables in output. Validate columns, types, completeness. Auto-repair and export.
 * 
 * Usage: npx @stockyard/mcp-tableforge
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("tableforge");
server.start();
