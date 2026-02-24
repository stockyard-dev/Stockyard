#!/usr/bin/env node
/**
 * @stockyard/mcp-canarydeploy — Canary deployments for prompt/model changes
 * 
 * MCP server for Stockyard CanaryDeploy.
 * Gradual rollout: 5%→25%→100%. Auto-promote if quality holds. Auto-rollback.
 * 
 * Usage: npx @stockyard/mcp-canarydeploy
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("canarydeploy");
server.start();
