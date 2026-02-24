#!/usr/bin/env node
/**
 * @stockyard/mcp-errornorm — Normalize error responses across providers
 * 
 * MCP server for Stockyard ErrorNorm.
 * Single error schema: code, message, provider, retry_after, is_retryable.
 * 
 * Usage: npx @stockyard/mcp-errornorm
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("errornorm");
server.start();
