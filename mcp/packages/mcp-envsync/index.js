#!/usr/bin/env node
/**
 * @stockyard/mcp-envsync — Sync configs + secrets across environments
 * 
 * MCP server for Stockyard EnvSync.
 * Push/promote/diff. Encrypted secrets. Pre-promotion validation. Rollback.
 * 
 * Usage: npx @stockyard/mcp-envsync
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("envsync");
server.start();
