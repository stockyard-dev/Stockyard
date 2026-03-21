import OpenAIOriginal from 'openai';

const STOCKYARD_URL = process.env.STOCKYARD_URL || 'http://localhost:4200';
const ENABLED = (process.env.STOCKYARD_ENABLED || 'true').toLowerCase() !== 'false';

/**
 * Drop-in replacement for the OpenAI client that routes through Stockyard.
 *
 * Usage:
 *   import OpenAI from '@stockyard/openai';
 *   const client = new OpenAI({ apiKey: 'sk-...' });
 */
export default class OpenAI extends OpenAIOriginal {
  constructor(opts: ConstructorParameters<typeof OpenAIOriginal>[0] = {}) {
    if (ENABLED && !opts.baseURL) {
      opts.baseURL = `${STOCKYARD_URL}/v1`;
      opts.defaultHeaders = {
        ...opts.defaultHeaders,
        'X-Stockyard-Source': 'sdk-node',
      };
    }
    super(opts);
  }
}

// Re-export all types and utilities
export * from 'openai';
