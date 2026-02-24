#!/usr/bin/env node
/**
 * @stockyard/mcp-extractml — Turn unstructured LLM responses into structured data
 * 
 * MCP server for Stockyard ExtractML.
 * Force extraction from free-text into JSON when models return prose.
 * 
 * Usage: npx @stockyard/mcp-extractml
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("extractml");
server.start();
