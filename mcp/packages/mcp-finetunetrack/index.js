#!/usr/bin/env node
/**
 * @stockyard/mcp-finetunetrack — Monitor fine-tuned model performance
 * 
 * MCP server for Stockyard FineTuneTrack.
 * Eval suite. Run periodically. Track scores. Compare to base model.
 * 
 * Usage: npx @stockyard/mcp-finetunetrack
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("finetunetrack");
server.start();
