# @stockyard/openai

Drop-in OpenAI SDK wrapper that routes all traffic through Stockyard.

## Installation

```bash
npm install @stockyard/openai
```

## Usage

Change one import line:

```typescript
// Before
import OpenAI from 'openai';

// After
import OpenAI from '@stockyard/openai';
```

Everything else works exactly the same:

```typescript
const client = new OpenAI({ apiKey: 'sk-...' });
const response = await client.chat.completions.create({
  model: 'gpt-4o',
  messages: [{ role: 'user', content: 'Hello' }],
});
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `STOCKYARD_URL` | `http://localhost:7749` | Stockyard proxy URL |
| `STOCKYARD_ENABLED` | `true` | Set to `false` to bypass Stockyard |
