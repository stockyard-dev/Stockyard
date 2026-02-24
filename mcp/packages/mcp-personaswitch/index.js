#!/usr/bin/env node
/**
 * @stockyard/mcp-personaswitch — Hot-swap AI personalities without code changes
 * 
 * MCP server for Stockyard PersonaSwitch.
 * Define personas. Route by header/key/segment. Each: prompt, temperature, rules.
 * 
 * Usage: npx @stockyard/mcp-personaswitch
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("personaswitch");
server.start();
