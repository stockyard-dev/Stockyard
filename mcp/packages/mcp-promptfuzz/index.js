#!/usr/bin/env node
/**
 * @stockyard/mcp-promptfuzz — Fuzz-test your prompts
 * 
 * MCP server for Stockyard PromptFuzz.
 * Generate adversarial, multilingual, edge-case inputs. Score with EvalGate. Report failures.
 * 
 * Usage: npx @stockyard/mcp-promptfuzz
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("promptfuzz");
server.start();
