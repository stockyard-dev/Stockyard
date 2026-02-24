#!/usr/bin/env node
/**
 * @stockyard/mcp-webhookforge — Visual builder for webhook→LLM→action pipelines
 * 
 * MCP server for Stockyard WebhookForge.
 * Visual flow builder. Trigger→transform→LLM→condition→action. History.
 * 
 * Usage: npx @stockyard/mcp-webhookforge
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("webhookforge");
server.start();
