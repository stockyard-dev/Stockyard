#!/usr/bin/env node
/**
 * @stockyard/mcp-scopeguard — Fine-grained permissions per API key
 * 
 * MCP server for Stockyard ScopeGuard.
 * Role-based access control. Map keys to allowed models, endpoints, features.
 * 
 * Usage: npx @stockyard/mcp-scopeguard
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("scopeguard");
server.start();
