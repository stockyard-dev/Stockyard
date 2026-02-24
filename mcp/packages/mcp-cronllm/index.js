#!/usr/bin/env node
/**
 * @stockyard/mcp-cronllm — Scheduled LLM tasks — your AI cron job runner
 * 
 * MCP server for Stockyard CronLLM.
 * Define scheduled prompts in YAML. Daily summaries, weekly reports, periodic checks. Runs through full proxy chain.
 * 
 * Usage: npx @stockyard/mcp-cronllm
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("cronllm");
server.start();
