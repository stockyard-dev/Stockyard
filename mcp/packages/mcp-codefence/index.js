#!/usr/bin/env node
/**
 * @stockyard/mcp-codefence — Validate LLM-generated code before it runs
 * 
 * MCP server for Stockyard CodeFence.
 * Scan LLM code output for dangerous patterns: shell injection, file access, crypto mining. Block or flag unsafe code.
 * 
 * Usage: npx @stockyard/mcp-codefence
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("codefence");
server.start();
