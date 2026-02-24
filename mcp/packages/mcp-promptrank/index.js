#!/usr/bin/env node
/**
 * @stockyard/mcp-promptrank — Rank prompts by ROI
 * 
 * MCP server for Stockyard PromptRank.
 * Per template: cost, quality, latency, volume, feedback. ROI leaderboard.
 * 
 * Usage: npx @stockyard/mcp-promptrank
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("promptrank");
server.start();
