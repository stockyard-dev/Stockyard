#!/usr/bin/env node
/**
 * @stockyard/mcp-audioproxy — Proxy for speech-to-text and text-to-speech
 * 
 * MCP server for Stockyard AudioProxy.
 * Cache TTS, track per-minute costs, failover between STT/TTS providers.
 * 
 * Usage: npx @stockyard/mcp-audioproxy
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("audioproxy");
server.start();
