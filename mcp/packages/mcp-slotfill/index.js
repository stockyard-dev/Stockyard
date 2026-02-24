#!/usr/bin/env node
/**
 * @stockyard/mcp-slotfill — Form-filling conversation engine
 * 
 * MCP server for Stockyard SlotFill.
 * Declarative slot definitions. Track filled/missing. Reprompt. Completion funnels.
 * 
 * Usage: npx @stockyard/mcp-slotfill
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("slotfill");
server.start();
