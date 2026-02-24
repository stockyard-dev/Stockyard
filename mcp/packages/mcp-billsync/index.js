#!/usr/bin/env node
/**
 * @stockyard/mcp-billsync — Per-customer LLM invoices automatically
 * 
 * MCP server for Stockyard BillSync.
 * Track usage per tenant. Apply markup. Generate invoice data. Stripe-compatible usage records.
 * 
 * Usage: npx @stockyard/mcp-billsync
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("billsync");
server.start();
