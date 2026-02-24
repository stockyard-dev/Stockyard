#!/usr/bin/env node
/**
 * @stockyard/mcp-summarizegate — Auto-summarize long contexts to save tokens
 * 
 * MCP server for Stockyard SummarizeGate.
 * Score relevance per section. Keep high-relevance verbatim. Summarize low-relevance.
 * 
 * Usage: npx @stockyard/mcp-summarizegate
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("summarizegate");
server.start();
