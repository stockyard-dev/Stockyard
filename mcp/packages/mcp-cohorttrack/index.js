#!/usr/bin/env node
/**
 * @stockyard/mcp-cohorttrack — User cohort analytics for LLM products
 * 
 * MCP server for Stockyard CohortTrack.
 * Cohorts by signup, plan, feature. Retention, cost per cohort. BI export.
 * 
 * Usage: npx @stockyard/mcp-cohorttrack
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("cohorttrack");
server.start();
