#!/usr/bin/env node
/**
 * @stockyard/mcp-retentionwipe — Automated data retention and deletion
 * 
 * MCP server for Stockyard RetentionWipe.
 * Retention periods per data type. Auto-purge. Per-user deletion. Deletion certificates.
 * 
 * Usage: npx @stockyard/mcp-retentionwipe
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("retentionwipe");
server.start();
