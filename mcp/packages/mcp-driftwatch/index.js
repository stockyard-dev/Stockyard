#!/usr/bin/env node
/**
 * @stockyard/mcp-driftwatch — Detect when model behavior changes
 * 
 * MCP server for Stockyard DriftWatch.
 * Track latency and output patterns per model over time. Alert when behavior drifts beyond thresholds.
 * 
 * Usage: npx @stockyard/mcp-driftwatch
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("driftwatch");
server.start();
