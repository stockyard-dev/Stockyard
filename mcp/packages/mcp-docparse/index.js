#!/usr/bin/env node
/**
 * @stockyard/mcp-docparse — Preprocess documents before they hit the LLM
 * 
 * MCP server for Stockyard DocParse.
 * PDF/Word/HTML text extraction. Smart chunking. Clean artifacts.
 * 
 * Usage: npx @stockyard/mcp-docparse
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("docparse");
server.start();
