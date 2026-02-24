#!/usr/bin/env node
/**
 * @stockyard/mcp-trainexport — Export LLM conversations as fine-tuning datasets
 * 
 * MCP server for Stockyard TrainExport.
 * Collect input/output pairs from live traffic. Export as OpenAI JSONL, Anthropic, or Alpaca format.
 * 
 * Usage: npx @stockyard/mcp-trainexport
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("trainexport");
server.start();
