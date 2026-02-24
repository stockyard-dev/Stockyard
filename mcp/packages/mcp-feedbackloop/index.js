#!/usr/bin/env node
/**
 * @stockyard/mcp-feedbackloop — Close the LLM improvement loop
 * 
 * MCP server for Stockyard FeedbackLoop.
 * Collect user ratings and feedback linked to specific LLM requests. Track quality trends over time.
 * 
 * Usage: npx @stockyard/mcp-feedbackloop
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("feedbackloop");
server.start();
