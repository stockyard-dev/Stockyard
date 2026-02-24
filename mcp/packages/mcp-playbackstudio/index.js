#!/usr/bin/env node
/**
 * @stockyard/mcp-playbackstudio — Interactive playground for exploring logged interactions
 * 
 * MCP server for Stockyard PlaybackStudio.
 * Advanced filters. Conversation threads. Side-by-side. Bulk actions.
 * 
 * Usage: npx @stockyard/mcp-playbackstudio
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("playbackstudio");
server.start();
