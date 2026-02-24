#!/usr/bin/env node
/**
 * @stockyard/mcp-approvalgate — Require human approval for prompt changes
 * 
 * MCP server for Stockyard ApprovalGate.
 * Approval workflow for prompt modifications. Track who approved what and when. Audit trail included.
 * 
 * Usage: npx @stockyard/mcp-approvalgate
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("approvalgate");
server.start();
