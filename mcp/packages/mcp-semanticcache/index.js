#!/usr/bin/env node
/**
 * @stockyard/mcp-semanticcache — Cache hits for similar prompts, not just identical
 * 
 * MCP server for Stockyard SemanticCache.
 * Embed prompts. Cosine similarity. Configurable threshold. 10x hit rate.
 * 
 * Usage: npx @stockyard/mcp-semanticcache
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("semanticcache");
server.start();
