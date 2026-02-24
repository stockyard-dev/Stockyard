#!/usr/bin/env node
/**
 * @stockyard/mcp-encryptvault — End-to-end encryption for sensitive LLM payloads
 * 
 * MCP server for Stockyard EncryptVault.
 * AES-GCM encryption for sensitive fields. Customer-managed keys. HIPAA/SOC2 compliance ready.
 * 
 * Usage: npx @stockyard/mcp-encryptvault
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("encryptvault");
server.start();
