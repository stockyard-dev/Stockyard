#!/usr/bin/env node
/**
 * @stockyard/mcp-modelalias — Abstract away model names with aliases
 * 
 * MCP server for Stockyard ModelAlias.
 * Aliases: fast→gpt-4o-mini, smart→claude-sonnet. Change mapping, all apps update.
 * 
 * Usage: npx @stockyard/mcp-modelalias
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("modelalias");
server.start();
