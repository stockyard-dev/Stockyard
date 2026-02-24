#!/usr/bin/env node
/**
 * @stockyard/mcp-voicebridge — LLM middleware for voice/TTS pipelines
 * 
 * MCP server for Stockyard VoiceBridge.
 * Strip markdown, URLs, code blocks from responses. Convert to speakable prose for voice assistants.
 * 
 * Usage: npx @stockyard/mcp-voicebridge
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("voicebridge");
server.start();
