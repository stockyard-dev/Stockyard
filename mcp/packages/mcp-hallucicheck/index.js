#!/usr/bin/env node
/**
 * @stockyard/mcp-hallucicheck — Catch LLM hallucinations before your users do
 * 
 * MCP server for Stockyard HalluciCheck.
 * Validate URLs, emails, and citations in LLM responses. Flag or retry when models invent non-existent references.
 * 
 * Usage: npx @stockyard/mcp-hallucicheck
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("hallucicheck");
server.start();
