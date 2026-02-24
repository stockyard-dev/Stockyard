#!/usr/bin/env node
/**
 * @stockyard/mcp-langbridge — Cross-language translation for multilingual apps
 * 
 * MCP server for Stockyard LangBridge.
 * Auto-detect language, translate to English for model, translate response back. Seamless multilingual support.
 * 
 * Usage: npx @stockyard/mcp-langbridge
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("langbridge");
server.start();
