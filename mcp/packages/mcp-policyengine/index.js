#!/usr/bin/env node
/**
 * @stockyard/mcp-policyengine — Codify AI governance as enforceable rules
 * 
 * MCP server for Stockyard PolicyEngine.
 * YAML policy rules compiled to middleware. Audit log. Compliance rate dashboard.
 * 
 * Usage: npx @stockyard/mcp-policyengine
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("policyengine");
server.start();
