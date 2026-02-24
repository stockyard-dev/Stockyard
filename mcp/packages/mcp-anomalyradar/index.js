#!/usr/bin/env node
/**
 * @stockyard/mcp-anomalyradar — ML-powered anomaly detection
 * 
 * MCP server for Stockyard AnomalyRadar.
 * Build statistical baselines. Z-score deviation detection. Auto-adjusting thresholds.
 * 
 * Usage: npx @stockyard/mcp-anomalyradar
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("anomalyradar");
server.start();
