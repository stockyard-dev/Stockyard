#!/usr/bin/env node
/**
 * @stockyard/mcp-geminishim — Tame Gemini's quirks behind clean API
 * 
 * MCP server for Stockyard GeminiShim.
 * Handle Gemini safety filter blocks with auto-retry. Normalize token counts. OpenAI-compatible surface for Gemini.
 * 
 * Usage: npx @stockyard/mcp-geminishim
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("geminishim");
server.start();
