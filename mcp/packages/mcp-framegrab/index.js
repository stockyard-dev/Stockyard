#!/usr/bin/env node
/**
 * @stockyard/mcp-framegrab — Extract and analyze video frames through vision LLMs
 * 
 * MCP server for Stockyard FrameGrab.
 * Scene detection. Batch frames. Smart frame selection. Cost per frame.
 * 
 * Usage: npx @stockyard/mcp-framegrab
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("framegrab");
server.start();
