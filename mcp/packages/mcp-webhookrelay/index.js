#!/usr/bin/env node
/**
 * @stockyard/mcp-webhookrelay — Trigger LLM calls from any webhook
 * 
 * MCP server for Stockyard WebhookRelay.
 * Receive webhooks, extract data, build prompts, call LLM, send results. GitHub→summarize→Slack in one config.
 * 
 * Usage: npx @stockyard/mcp-webhookrelay
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("webhookrelay");
server.start();
