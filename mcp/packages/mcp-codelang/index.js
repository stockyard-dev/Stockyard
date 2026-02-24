#!/usr/bin/env node
/**
 * @stockyard/mcp-codelang — Language-aware code generation with syntax validation
 * 
 * MCP server for Stockyard CodeLang.
 * Tree-sitter parsing. Syntax errors, undefined refs, suspicious patterns.
 * 
 * Usage: npx @stockyard/mcp-codelang
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("codelang");
server.start();
