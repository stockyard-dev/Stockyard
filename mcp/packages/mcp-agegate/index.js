#!/usr/bin/env node
/**
 * @stockyard/mcp-agegate — Child safety middleware for LLM apps
 * 
 * MCP server for Stockyard AgeGate.
 * Age-appropriate content filtering. Tiers: child, teen, adult. Injects safety prompts, filters output. COPPA/KOSA ready.
 * 
 * Usage: npx @stockyard/mcp-agegate
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("agegate");
server.start();
