#!/usr/bin/env node
/**
 * @stockyard/mcp-paramnorm — Normalize parameters across providers
 * 
 * MCP server for Stockyard ParamNorm.
 * Calibration profiles per model. Map normalized params to model-specific ranges.
 * 
 * Usage: npx @stockyard/mcp-paramnorm
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("paramnorm");
server.start();
