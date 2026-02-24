#!/usr/bin/env node
/**
 * @stockyard/mcp-llmsync — Replicate config across environments
 * 
 * MCP server for Stockyard LLMSync.
 * Environment hierarchy with config inheritance. Diff, promote, rollback. Git-friendly YAML management.
 * 
 * Usage: npx @stockyard/mcp-llmsync
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("llmsync");
server.start();
