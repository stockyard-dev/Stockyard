#!/usr/bin/env node
/**
 * @stockyard/mcp-snapshottest — Snapshot testing for LLM outputs
 * 
 * MCP server for Stockyard SnapshotTest.
 * Record baselines. Semantic diff. Configurable threshold. CI-friendly.
 * 
 * Usage: npx @stockyard/mcp-snapshottest
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("snapshottest");
server.start();
