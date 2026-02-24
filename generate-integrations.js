#!/usr/bin/env node
/**
 * Generate platform integration configs for all 126 Stockyard integrations.
 * Skips directories that already exist.
 * 
 * Run: node generate-integrations.js
 */

const fs = require("fs");
const path = require("path");

const DIR = path.join(__dirname, "integrations");
let created = 0, skipped = 0;

function write(dir, file, content) {
  const d = path.join(DIR, dir);
  if (!fs.existsSync(d)) fs.mkdirSync(d, { recursive: true });
  fs.writeFileSync(path.join(d, file), content.trimStart() + "\n");
}

function readme(dir, name, category, type, desc, setupSteps, files) {
  const fileList = files.map(f => `- \`${f}\``).join("\n");
  write(dir, "README.md", `
# ${name} + Stockyard

> **Category:** ${category} | **Type:** ${type}

${desc}

## Quick Setup

${setupSteps}

## Files

${fileList}

## How It Works

All LLM requests from ${name} are routed through Stockyard's proxy at \`http://localhost:4000/v1\`. Stockyard handles cost tracking, caching, rate limiting, failover, and all other middleware — transparently.

Your ${name} setup doesn't need to change beyond pointing the base URL at Stockyard.

## Using Individual Products

Instead of the full suite (port 4000), you can point at individual products:

| Product | Port | What It Does |
|---------|------|-------------|
| CostCap | 4100 | Spending caps only |
| CacheLayer | 4200 | Response caching only |
| RateShield | 4500 | Rate limiting only |
| FallbackRouter | 4400 | Failover routing only |

## Learn More

- [Stockyard Docs](https://stockyard.dev/docs/)
- [All 125 Products](https://stockyard.dev/products/)
- [GitHub](https://github.com/stockyard-dev/stockyard)
`);
}

function gen(dir, name, cat, type, desc, fn) {
  if (fs.existsSync(path.join(DIR, dir, "README.md"))) {
    skipped++;
    return;
  }
  fn();
  created++;
}

// ═══════════════════════════════════════════════════════════════
// 3.1 AI CODING TOOLS & IDEs (10)
// ═══════════════════════════════════════════════════════════════

gen("cursor", "Cursor", "AI Coding Tools", "MCP + config", "Route all Cursor AI requests through Stockyard.", () => {
  write("cursor", "mcp.json", `
{
  "mcpServers": {
    "stockyard": {
      "command": "npx",
      "args": ["@stockyard/mcp-stockyard"],
      "env": {
        "OPENAI_API_KEY": "your-openai-key"
      }
    }
  }
}`);
  write("cursor", "setup.sh", `
#!/bin/bash
# Cursor + Stockyard one-line setup
mkdir -p ~/.cursor
cp mcp.json ~/.cursor/mcp.json
echo "Restart Cursor. Stockyard is now active."
echo "Dashboard: http://localhost:4000/ui"
`);
  readme("cursor", "Cursor", "AI Coding Tools", "MCP + config",
    "Route all Cursor AI requests through Stockyard for cost tracking, caching, and rate limiting. Every AI completion, edit, and chat goes through the proxy.",
    "1. Copy `mcp.json` to `~/.cursor/mcp.json`\n2. Set your `OPENAI_API_KEY`\n3. Restart Cursor\n4. Open dashboard at http://localhost:4000/ui",
    ["mcp.json", "setup.sh"]);
});

gen("continue-dev", "Continue.dev", "AI Coding Tools", "Config template", "Add Stockyard to Continue.dev for cost control.", () => {
  write("continue-dev", "config.json", `
{
  "models": [
    {
      "title": "GPT-4o via Stockyard",
      "provider": "openai",
      "model": "gpt-4o",
      "apiBase": "http://localhost:4000/v1",
      "apiKey": "any-string"
    },
    {
      "title": "Claude via Stockyard",
      "provider": "openai",
      "model": "claude-sonnet-4-20250514",
      "apiBase": "http://localhost:4000/v1",
      "apiKey": "any-string"
    }
  ],
  "tabAutocompleteModel": {
    "title": "Fast Autocomplete via Stockyard",
    "provider": "openai",
    "model": "gpt-4o-mini",
    "apiBase": "http://localhost:4000/v1",
    "apiKey": "any-string"
  }
}`);
  readme("continue-dev", "Continue.dev", "AI Coding Tools", "Config template",
    "Route Continue.dev completions and chat through Stockyard.",
    "1. Copy `config.json` to `~/.continue/config.json`\n2. Start Stockyard: `npx @stockyard/mcp-stockyard`\n3. Restart VS Code",
    ["config.json"]);
});

gen("aider", "Aider", "AI Coding Tools", "Env wrapper", "Run Aider through Stockyard.", () => {
  write("aider", "aider-stockyard.sh", `
#!/bin/bash
# Run Aider through Stockyard proxy
export OPENAI_API_BASE=http://localhost:4000/v1
export OPENAI_API_KEY=\${OPENAI_API_KEY:-any-string}
exec aider "$@"
`);
  write("aider", ".env", `
# Add to your shell profile or .env
OPENAI_API_BASE=http://localhost:4000/v1
OPENAI_API_KEY=your-key
`);
  readme("aider", "Aider", "AI Coding Tools", "Env wrapper",
    "Route Aider AI pair programming through Stockyard for cost tracking and model failover.",
    "1. Start Stockyard: `npx @stockyard/mcp-stockyard`\n2. `source .env`\n3. Run `aider` normally — all requests go through Stockyard",
    ["aider-stockyard.sh", ".env"]);
});

gen("cline", "Cline / Roo Code", "AI Coding Tools", "Settings + MCP", "Connect Cline to Stockyard.", () => {
  write("cline", "mcp.json", `
{
  "mcpServers": {
    "stockyard": {
      "command": "npx",
      "args": ["@stockyard/mcp-stockyard"],
      "env": { "OPENAI_API_KEY": "your-key" }
    }
  }
}`);
  write("cline", "settings.json", `
{
  "cline.apiProvider": "openai-compatible",
  "cline.openaiBaseUrl": "http://localhost:4000/v1",
  "cline.openaiApiKey": "any-string",
  "cline.openaiModelId": "gpt-4o"
}`);
  readme("cline", "Cline / Roo Code", "AI Coding Tools", "Settings + MCP",
    "Connect Cline (VS Code AI agent) to Stockyard for cost caps and request logging.",
    "1. Add `settings.json` values to VS Code settings\n2. Copy `mcp.json` to your project root\n3. Restart VS Code",
    ["mcp.json", "settings.json"]);
});

gen("windsurf", "Windsurf / Codeium", "AI Coding Tools", "Config template", "Route Windsurf through Stockyard.", () => {
  write("windsurf", "config.md", `
# Windsurf + Stockyard

In Windsurf settings:
- AI Provider: **OpenAI Compatible**
- Base URL: **http://localhost:4000/v1**
- API Key: **any-string**
- Model: **gpt-4o**
`);
  readme("windsurf", "Windsurf / Codeium", "AI Coding Tools", "Config template",
    "Route Windsurf AI requests through Stockyard for spend tracking and caching.",
    "1. Start Stockyard: `npx @stockyard/mcp-stockyard`\n2. Open Windsurf Settings > AI Provider\n3. Set Base URL to `http://localhost:4000/v1`",
    ["config.md"]);
});

gen("github-copilot", "GitHub Copilot", "AI Coding Tools", "Network proxy", "Analytics for GitHub Copilot via network proxy.", () => {
  write("github-copilot", "vscode-settings.json", `
{
  "http.proxy": "http://localhost:4000",
  "http.proxyStrictSSL": false
}`);
  readme("github-copilot", "GitHub Copilot", "AI Coding Tools", "Network proxy",
    "Route GitHub Copilot through Stockyard via HTTP proxy for analytics and logging. Note: Copilot manages its own auth — Stockyard provides visibility only.",
    "1. Start Stockyard: `npx @stockyard/mcp-stockyard`\n2. Add proxy settings to VS Code\n3. Copilot traffic now logged in Stockyard dashboard",
    ["vscode-settings.json"]);
});

gen("zed", "Zed", "AI Coding Tools", "Settings template", "Configure Zed AI to route through Stockyard.", () => {
  write("zed", "settings.json", `
{
  "language_models": {
    "openai": {
      "api_url": "http://localhost:4000/v1",
      "available_models": [
        { "name": "gpt-4o", "display_name": "GPT-4o via Stockyard" },
        { "name": "gpt-4o-mini", "display_name": "GPT-4o Mini via Stockyard" }
      ]
    }
  }
}`);
  readme("zed", "Zed", "AI Coding Tools", "Settings template",
    "Configure Zed editor's built-in AI to route through Stockyard.",
    "1. Start Stockyard\n2. Copy `settings.json` to `~/.config/zed/settings.json`\n3. Restart Zed",
    ["settings.json"]);
});

gen("jetbrains", "JetBrains AI", "AI Coding Tools", "Proxy config", "Route JetBrains AI through Stockyard.", () => {
  write("jetbrains", "setup.md", `
# JetBrains + Stockyard

For JetBrains with OpenAI-compatible plugins:

1. Settings > Tools > AI Assistant > Custom provider
2. Base URL: http://localhost:4000/v1
3. API Key: any-string

For HTTP proxy approach:
1. Settings > Appearance & Behavior > System Settings > HTTP Proxy
2. Manual proxy: localhost:4000
`);
  readme("jetbrains", "JetBrains AI", "AI Coding Tools", "Proxy config",
    "Route JetBrains AI Assistant through Stockyard for cost control.",
    "1. Start Stockyard\n2. Configure proxy in JetBrains settings\n3. See `setup.md` for detailed steps",
    ["setup.md"]);
});

gen("neovim", "Neovim / avante.nvim", "AI Coding Tools", "Config snippets", "Configure Neovim AI plugins with Stockyard.", () => {
  write("neovim", "avante.lua", `
-- ~/.config/nvim/lua/plugins/avante.lua
return {
  "yetone/avante.nvim",
  opts = {
    provider = "openai",
    openai = {
      endpoint = "http://localhost:4000/v1",
      model = "gpt-4o",
      api_key_name = "OPENAI_API_KEY",
    },
  },
}`);
  write("neovim", "codecompanion.lua", `
-- For codecompanion.nvim
require("codecompanion").setup({
  adapters = {
    openai = function()
      return require("codecompanion.adapters").extend("openai", {
        url = "http://localhost:4000/v1/chat/completions",
      })
    end,
  },
})`);
  readme("neovim", "Neovim / avante.nvim", "AI Coding Tools", "Config snippets",
    "Configure avante.nvim and codecompanion.nvim to use Stockyard.",
    "1. Start Stockyard\n2. Copy the relevant Lua config to your Neovim setup\n3. Restart Neovim",
    ["avante.lua", "codecompanion.lua"]);
});

gen("opencode", "OpenCode", "AI Coding Tools", "Config + env", "Configure OpenCode terminal AI with Stockyard.", () => {
  write("opencode", ".env", `
OPENAI_API_BASE=http://localhost:4000/v1
OPENAI_API_KEY=your-key
`);
  readme("opencode", "OpenCode", "AI Coding Tools", "Config + env",
    "Route OpenCode terminal AI through Stockyard.",
    "1. Start Stockyard\n2. Set env vars: `export OPENAI_API_BASE=http://localhost:4000/v1`\n3. Run OpenCode normally",
    [".env"]);
});

// ═══════════════════════════════════════════════════════════════
// 3.2 AI APPLICATION FRAMEWORKS (10)
// ═══════════════════════════════════════════════════════════════

gen("vercel-ai", "Vercel AI SDK", "AI Frameworks", "npm provider", "Drop-in Stockyard provider for Vercel AI SDK.", () => {
  write("vercel-ai", "route.ts", `
// app/api/chat/route.ts
import { createOpenAI } from "@ai-sdk/openai";
import { streamText } from "ai";

const stockyard = createOpenAI({
  baseURL: "http://localhost:4000/v1",
  apiKey: "any-string",
});

export async function POST(req: Request) {
  const { messages } = await req.json();
  const result = streamText({ model: stockyard("gpt-4o"), messages });
  return result.toDataStreamResponse();
}`);
  readme("vercel-ai", "Vercel AI SDK", "AI Frameworks", "npm provider",
    "Drop-in Stockyard provider for the Vercel AI SDK. Works with Next.js, SvelteKit, Nuxt.",
    "1. `npm install @ai-sdk/openai ai`\n2. Copy `route.ts` to your API route\n3. Start Stockyard on port 4000",
    ["route.ts"]);
});

gen("llamaindex", "LlamaIndex", "AI Frameworks", "PyPI package", "Route LlamaIndex through Stockyard.", () => {
  write("llamaindex", "example.py", `
# pip install llama-index-llms-openai llama-index-embeddings-openai
from llama_index.llms.openai import OpenAI
from llama_index.embeddings.openai import OpenAIEmbedding

llm = OpenAI(
    model="gpt-4o",
    api_base="http://localhost:4000/v1",
    api_key="any-string",
)

embed = OpenAIEmbedding(
    model="text-embedding-3-small",
    api_base="http://localhost:4000/v1",
    api_key="any-string",
)`);
  readme("llamaindex", "LlamaIndex", "AI Frameworks", "PyPI package",
    "Route LlamaIndex completions and embeddings through Stockyard. Embeddings are automatically cached.",
    "1. Start Stockyard\n2. Set `api_base` to `http://localhost:4000/v1`\n3. Embeddings cached automatically via CacheLayer",
    ["example.py"]);
});

gen("haystack", "Haystack", "AI Frameworks", "PyPI package", "Configure Haystack RAG pipelines with Stockyard.", () => {
  write("haystack", "example.py", `
from haystack.components.generators.chat import OpenAIChatGenerator
from haystack.utils import Secret

generator = OpenAIChatGenerator(
    api_key=Secret.from_token("any-string"),
    api_base_url="http://localhost:4000/v1",
    model="gpt-4o",
)`);
  readme("haystack", "Haystack", "AI Frameworks", "PyPI package",
    "Configure Haystack RAG pipelines to route through Stockyard.",
    "1. `pip install haystack-ai`\n2. Set `api_base_url` to Stockyard\n3. All Haystack LLM calls now go through the proxy",
    ["example.py"]);
});

gen("semantic-kernel", "Semantic Kernel", "AI Frameworks", "NuGet + PyPI", "Use Stockyard with Microsoft Semantic Kernel.", () => {
  write("semantic-kernel", "example.cs", `
using Microsoft.SemanticKernel;

var builder = Kernel.CreateBuilder();
builder.AddOpenAIChatCompletion(
    modelId: "gpt-4o",
    endpoint: new Uri("http://localhost:4000/v1"),
    apiKey: "any-string"
);
var kernel = builder.Build();`);
  write("semantic-kernel", "example.py", `
import semantic_kernel as sk
from semantic_kernel.connectors.ai.open_ai import OpenAIChatCompletion

kernel = sk.Kernel()
kernel.add_service(OpenAIChatCompletion(
    ai_model_id="gpt-4o",
    async_client=None,
    api_key="any-string",
    org_id=None,
    default_headers=None,
    # Set base URL via environment: OPENAI_BASE_URL=http://localhost:4000/v1
))`);
  readme("semantic-kernel", "Semantic Kernel", "AI Frameworks", "NuGet + PyPI",
    "Use Stockyard with Microsoft's Semantic Kernel for .NET and Python.",
    "1. Start Stockyard\n2. Set endpoint to `http://localhost:4000/v1`\n3. See examples for C# and Python",
    ["example.cs", "example.py"]);
});

gen("spring-ai", "Spring AI", "AI Frameworks", "Maven starter", "Configure Spring AI with Stockyard.", () => {
  write("spring-ai", "application.yml", `
spring:
  ai:
    openai:
      base-url: http://localhost:4000/v1
      api-key: any-string
      chat:
        options:
          model: gpt-4o`);
  write("spring-ai", "pom-snippet.xml", `
<!-- Add to pom.xml dependencies -->
<dependency>
  <groupId>org.springframework.ai</groupId>
  <artifactId>spring-ai-openai-spring-boot-starter</artifactId>
</dependency>`);
  readme("spring-ai", "Spring AI", "AI Frameworks", "Maven starter",
    "Configure Spring AI to route through Stockyard. Java enterprise with LLM cost control.",
    "1. Add Spring AI OpenAI starter to `pom.xml`\n2. Set `spring.ai.openai.base-url` in `application.yml`\n3. Start Stockyard on port 4000",
    ["application.yml", "pom-snippet.xml"]);
});

gen("litellm-compat", "LiteLLM", "AI Frameworks", "Drop-in replacement", "Migrate from LiteLLM to Stockyard.", () => {
  write("litellm-compat", "migration-guide.md", `
# Migrating from LiteLLM to Stockyard

## Why Migrate?
- LiteLLM: Python, needs Redis/Postgres, 35K+ stars but heavy
- Stockyard: Go single binary, SQLite only, no dependencies

## Step 1: Replace LiteLLM proxy with Stockyard
\`\`\`bash
# Before: litellm --port 4000
# After:
npx @stockyard/mcp-stockyard
\`\`\`

## Step 2: Same base URL, same API
Your code doesn't change. Stockyard speaks OpenAI-compatible API.

\`\`\`python
# This works with both LiteLLM and Stockyard:
from openai import OpenAI
client = OpenAI(base_url="http://localhost:4000/v1", api_key="any")
\`\`\`

## Step 3: Migrate config
\`\`\`yaml
# stockyard.yml
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
  anthropic:
    api_key: \${ANTHROPIC_API_KEY}
\`\`\`

## What You Gain
- 125 middleware products vs LiteLLM's proxy-only approach
- No Python runtime, no Redis, no Postgres
- 6MB static binary vs Python environment
- Embedded dashboard (no separate UI service)
`);
  readme("litellm-compat", "LiteLLM", "AI Frameworks", "Drop-in replacement",
    "Migrate from LiteLLM to Stockyard. Same OpenAI-compatible API, single Go binary, no dependencies.",
    "1. Stop LiteLLM proxy\n2. Start Stockyard: `npx @stockyard/mcp-stockyard`\n3. Same base URL — your code doesn't change",
    ["migration-guide.md"]);
});

gen("instructor", "Instructor / Pydantic AI", "AI Frameworks", "Config guide", "Use Instructor structured extraction with Stockyard.", () => {
  write("instructor", "example.py", `
import instructor
from openai import OpenAI

client = instructor.from_openai(
    OpenAI(
        base_url="http://localhost:4000/v1",
        api_key="any-string",
    )
)

# Stockyard's StructuredShield validates JSON automatically
user = client.chat.completions.create(
    model="gpt-4o",
    response_model=User,
    messages=[{"role": "user", "content": "Extract: John is 25"}],
)`);
  readme("instructor", "Instructor / Pydantic AI", "AI Frameworks", "Config guide",
    "Use Instructor structured extraction through Stockyard. Pairs naturally with StructuredShield.",
    "1. `pip install instructor openai`\n2. Point OpenAI client at Stockyard\n3. StructuredShield adds automatic JSON validation on top",
    ["example.py"]);
});

gen("dspy", "DSPy", "AI Frameworks", "PyPI module", "Route DSPy optimization calls through Stockyard.", () => {
  write("dspy", "example.py", `
import dspy

# DSPy makes thousands of LLM calls during optimization.
# Stockyard caching + cost caps are essential.
lm = dspy.LM(
    model="openai/gpt-4o-mini",
    api_base="http://localhost:4000/v1",
    api_key="any-string",
)
dspy.configure(lm=lm)`);
  readme("dspy", "DSPy", "AI Frameworks", "PyPI module",
    "Route DSPy optimization calls through Stockyard. DSPy makes thousands of LLM calls per optimization run — caching and cost caps are essential.",
    "1. Start Stockyard with CostCap enabled\n2. Set `api_base` in DSPy LM config\n3. CacheLayer dramatically reduces optimization cost",
    ["example.py"]);
});

gen("guidance", "Guidance", "AI Frameworks", "Config guide", "Use Microsoft Guidance with Stockyard.", () => {
  write("guidance", "example.py", `
import guidance

# Set OpenAI base URL to Stockyard
import os
os.environ["OPENAI_API_BASE"] = "http://localhost:4000/v1"
os.environ["OPENAI_API_KEY"] = "any-string"

model = guidance.models.OpenAI("gpt-4o")`);
  readme("guidance", "Guidance", "AI Frameworks", "Config guide",
    "Use Microsoft Guidance constrained generation through Stockyard.",
    "1. Set `OPENAI_API_BASE=http://localhost:4000/v1`\n2. Use Guidance normally\n3. StructuredShield adds validation on top",
    ["example.py"]);
});

gen("mirascope", "Mirascope", "AI Frameworks", "Config guide", "Route Mirascope calls through Stockyard.", () => {
  write("mirascope", "example.py", `
from mirascope.core import openai

# Point at Stockyard
import os
os.environ["OPENAI_API_BASE"] = "http://localhost:4000/v1"

@openai.call("gpt-4o")
def recommend_book(genre: str) -> str:
    return f"Recommend a {genre} book"`);
  readme("mirascope", "Mirascope", "AI Frameworks", "Config guide",
    "Route Mirascope Python LLM calls through Stockyard.",
    "1. Set `OPENAI_API_BASE` environment variable\n2. Use Mirascope normally",
    ["example.py"]);
});

// ═══════════════════════════════════════════════════════════════
// 3.3 WORKFLOW & AUTOMATION (10)
// ═══════════════════════════════════════════════════════════════

gen("zapier", "Zapier", "Workflow", "Zapier app", "Trigger Stockyard actions from Zapier workflows.", () => {
  write("zapier", "setup.md", `
# Zapier + Stockyard

## Option 1: Webhooks by Zapier
1. Use "Webhooks by Zapier" action
2. Method: POST
3. URL: http://your-stockyard-host:4000/v1/chat/completions
4. Headers: Content-Type: application/json
5. Body: {"model":"gpt-4o","messages":[{"role":"user","content":"{{input}}"}]}

## Option 2: OpenAI Integration with Custom URL
Some Zapier OpenAI actions support custom base URLs.
Set to your Stockyard instance URL.
`);
  readme("zapier", "Zapier", "Workflow", "Zapier app",
    "Trigger LLM calls through Stockyard from Zapier workflows. Cost-capped AI automation.",
    "1. Deploy Stockyard to a public URL (Railway/Render/Fly.io)\n2. Use Webhooks by Zapier to call the API\n3. See `setup.md` for details",
    ["setup.md"]);
});

gen("make", "Make.com", "Workflow", "Custom module", "Connect Make.com scenarios to Stockyard.", () => {
  write("make", "setup.md", `
# Make.com + Stockyard

1. Add an HTTP module to your scenario
2. URL: http://your-stockyard-host:4000/v1/chat/completions
3. Method: POST
4. Headers: Content-Type: application/json
5. Body: {"model":"gpt-4o","messages":[{"role":"user","content":"{{input}}"}]}
6. Parse response: choices[0].message.content
`);
  readme("make", "Make.com", "Workflow", "Custom module",
    "Connect Make.com automation scenarios to Stockyard.",
    "1. Deploy Stockyard publicly\n2. Add HTTP module pointing at Stockyard\n3. All Make.com LLM calls are now tracked",
    ["setup.md"]);
});

gen("pipedream", "Pipedream", "Workflow", "Node.js component", "Use Stockyard in Pipedream workflows.", () => {
  write("pipedream", "stockyard-step.mjs", `
// Pipedream Node.js step
import { OpenAI } from "openai";

export default defineComponent({
  async run({ steps, $ }) {
    const client = new OpenAI({
      baseURL: "http://your-stockyard-host:4000/v1",
      apiKey: "any-string",
    });
    const response = await client.chat.completions.create({
      model: "gpt-4o",
      messages: [{ role: "user", content: steps.trigger.event.body.prompt }],
    });
    return response.choices[0].message.content;
  },
});`);
  readme("pipedream", "Pipedream", "Workflow", "Node.js component",
    "Use Stockyard in Pipedream developer workflows.",
    "1. Deploy Stockyard\n2. Add Node.js step with OpenAI SDK pointed at Stockyard\n3. See `stockyard-step.mjs` for template",
    ["stockyard-step.mjs"]);
});

gen("activepieces", "Activepieces", "Workflow", "TypeScript connector", "Open-source Zapier alternative with Stockyard.", () => {
  write("activepieces", "setup.md", `
# Activepieces + Stockyard

Use the HTTP piece to call Stockyard:
1. Method: POST
2. URL: http://localhost:4000/v1/chat/completions
3. Headers: {"Content-Type": "application/json"}
4. Body: {"model":"gpt-4o","messages":[{"role":"user","content":"{{trigger.body}}"}]}
`);
  readme("activepieces", "Activepieces", "Workflow", "TypeScript connector",
    "Connect Activepieces (open-source Zapier) to Stockyard.",
    "1. Start Stockyard\n2. Use HTTP piece to call the API\n3. All requests tracked in dashboard",
    ["setup.md"]);
});

gen("temporal", "Temporal / Inngest", "Workflow", "Activity definitions", "Durable LLM workflows with Stockyard.", () => {
  write("temporal", "activities.go", `
package stockyard

import (
	"context"
	"github.com/sashabaranov/go-openai"
)

// StockyardActivity wraps LLM calls through Stockyard proxy
func StockyardActivity(ctx context.Context, prompt string) (string, error) {
	client := openai.NewClientWithConfig(openai.ClientConfig{
		BaseURL: "http://localhost:4000/v1",
	})
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    "gpt-4o",
		Messages: []openai.ChatCompletionMessage{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return "", err
	}
	return resp.Choices[0].Message.Content, nil
}`);
  write("temporal", "inngest-example.ts", `
import { inngest } from "./client";
import OpenAI from "openai";

const client = new OpenAI({ baseURL: "http://localhost:4000/v1", apiKey: "any" });

export const summarize = inngest.createFunction(
  { id: "summarize" },
  { event: "doc/uploaded" },
  async ({ event, step }) => {
    const result = await step.run("llm-call", async () => {
      const r = await client.chat.completions.create({
        model: "gpt-4o",
        messages: [{ role: "user", content: \`Summarize: \${event.data.text}\` }],
      });
      return r.choices[0].message.content;
    });
    return { summary: result };
  }
);`);
  readme("temporal", "Temporal / Inngest", "Workflow", "Activity definitions",
    "Build durable LLM workflows with Temporal or Inngest, routed through Stockyard.",
    "1. Start Stockyard\n2. Use the Go activity or Inngest step examples\n3. Retries, cost caps, and caching handled by Stockyard",
    ["activities.go", "inngest-example.ts"]);
});

gen("windmill", "Windmill", "Workflow", "Resource + templates", "Use Stockyard in Windmill scripts.", () => {
  write("windmill", "example.py", `
# Windmill Python script with Stockyard
import openai

def main(prompt: str):
    client = openai.OpenAI(
        base_url="http://stockyard:4000/v1",
        api_key="any-string",
    )
    r = client.chat.completions.create(
        model="gpt-4o",
        messages=[{"role": "user", "content": prompt}],
    )
    return r.choices[0].message.content`);
  readme("windmill", "Windmill", "Workflow", "Resource + templates",
    "Use Stockyard in Windmill (open-source Retool for scripts).",
    "1. Deploy Stockyard alongside Windmill\n2. Create OpenAI resource pointing at Stockyard\n3. All scripts use cost-capped LLM calls",
    ["example.py"]);
});

gen("node-red", "Node-RED", "Workflow", "npm nodes", "Visual LLM workflows through Stockyard.", () => {
  write("node-red", "flow.json", `
[
  {
    "id": "stockyard-llm",
    "type": "http request",
    "name": "Stockyard LLM",
    "method": "POST",
    "url": "http://localhost:4000/v1/chat/completions",
    "headers": { "Content-Type": "application/json" },
    "payload": "{\"model\":\"gpt-4o\",\"messages\":[{\"role\":\"user\",\"content\":\"{{payload}}\"}]}"
  }
]`);
  readme("node-red", "Node-RED", "Workflow", "npm nodes",
    "Build visual LLM flows in Node-RED through Stockyard.",
    "1. Import `flow.json` into Node-RED\n2. Start Stockyard on localhost:4000\n3. Connect input/output nodes",
    ["flow.json"]);
});

gen("airflow", "Apache Airflow", "Workflow", "PyPI provider", "Batch LLM pipelines with Airflow + Stockyard.", () => {
  write("airflow", "stockyard_operator.py", `
from airflow.models import BaseOperator
from openai import OpenAI

class StockyardOperator(BaseOperator):
    def __init__(self, prompt, model="gpt-4o", **kwargs):
        super().__init__(**kwargs)
        self.prompt = prompt
        self.model = model

    def execute(self, context):
        client = OpenAI(
            base_url="http://stockyard:4000/v1",
            api_key="any-string",
        )
        r = client.chat.completions.create(
            model=self.model,
            messages=[{"role": "user", "content": self.prompt}],
        )
        return r.choices[0].message.content`);
  readme("airflow", "Apache Airflow", "Workflow", "PyPI provider",
    "Run batch LLM data pipelines with Airflow, routed through Stockyard for cost control.",
    "1. Deploy Stockyard in your Airflow network\n2. Use `StockyardOperator` in DAGs\n3. BatchQueue handles concurrency",
    ["stockyard_operator.py"]);
});

gen("prefect", "Prefect", "Workflow", "PyPI package", "Prefect flows with Stockyard LLM calls.", () => {
  write("prefect", "example.py", `
from prefect import flow, task
from openai import OpenAI

client = OpenAI(base_url="http://localhost:4000/v1", api_key="any")

@task
def summarize(text: str) -> str:
    r = client.chat.completions.create(
        model="gpt-4o",
        messages=[{"role": "user", "content": f"Summarize: {text}"}],
    )
    return r.choices[0].message.content

@flow
def process_docs(docs: list[str]):
    return [summarize(d) for d in docs]`);
  readme("prefect", "Prefect", "Workflow", "PyPI package",
    "Use Stockyard in Prefect data pipeline flows.",
    "1. Start Stockyard\n2. Point OpenAI client at `http://localhost:4000/v1`\n3. All Prefect LLM tasks get caching + cost caps",
    ["example.py"]);
});

gen("dagster", "Dagster", "Workflow", "PyPI package", "Dagster asset pipelines with Stockyard.", () => {
  write("dagster", "example.py", `
from dagster import asset
from openai import OpenAI

client = OpenAI(base_url="http://localhost:4000/v1", api_key="any")

@asset
def summarized_docs(raw_docs):
    results = []
    for doc in raw_docs:
        r = client.chat.completions.create(
            model="gpt-4o",
            messages=[{"role": "user", "content": f"Summarize: {doc}"}],
        )
        results.append(r.choices[0].message.content)
    return results`);
  readme("dagster", "Dagster", "Workflow", "PyPI package",
    "Build Dagster asset pipelines with Stockyard-routed LLM calls.",
    "1. Start Stockyard\n2. Use OpenAI SDK pointed at Stockyard in your assets\n3. BatchQueue recommended for large datasets",
    ["example.py"]);
});

// ═══════════════════════════════════════════════════════════════
// 3.4 CHAT & CONVERSATIONAL AI (7)
// ═══════════════════════════════════════════════════════════════

gen("botpress", "Botpress", "Chat Platforms", "Integration module", "Route Botpress LLM calls through Stockyard.", () => {
  write("botpress", "setup.md", `
# Botpress + Stockyard

In Botpress Studio:
1. Settings > AI > Custom LLM
2. Endpoint: http://your-stockyard-host:4000/v1/chat/completions
3. API Key: any-string
4. Model: gpt-4o

All Botpress AI features (intents, entities, generation) route through Stockyard.
`);
  readme("botpress", "Botpress", "Chat Platforms", "Integration module",
    "Route Botpress chatbot LLM calls through Stockyard for cost control.",
    "1. Deploy Stockyard\n2. Set custom LLM endpoint in Botpress Studio\n3. All bot AI calls are now tracked",
    ["setup.md"]);
});

gen("rasa", "Rasa", "Chat Platforms", "LLM connector", "Use Stockyard with Rasa conversational AI.", () => {
  write("rasa", "endpoints.yml", `
# endpoints.yml
llm:
  type: openai
  api_base: http://stockyard:4000/v1
  api_key: any-string
  model: gpt-4o`);
  readme("rasa", "Rasa", "Chat Platforms", "LLM connector",
    "Route Rasa enterprise conversational AI through Stockyard.",
    "1. Set `api_base` in `endpoints.yml`\n2. Start Stockyard alongside Rasa\n3. All LLM calls get cost tracking and caching",
    ["endpoints.yml"]);
});

gen("voiceflow", "Voiceflow", "Chat Platforms", "API integration", "Connect Voiceflow to Stockyard.", () => {
  write("voiceflow", "setup.md", `
# Voiceflow + Stockyard

1. In Voiceflow, use API Block or Custom Integration
2. Endpoint: http://your-stockyard-host:4000/v1/chat/completions
3. Method: POST
4. Pass conversation context as messages array
`);
  readme("voiceflow", "Voiceflow", "Chat Platforms", "API integration",
    "Connect Voiceflow visual conversation design to Stockyard.",
    "1. Deploy Stockyard publicly\n2. Use API Block in Voiceflow pointing at Stockyard\n3. Conversations get cost tracking",
    ["setup.md"]);
});

gen("chainlit", "Chainlit", "Chat Platforms", "Config guide", "Build Chainlit chat UIs with Stockyard backend.", () => {
  write("chainlit", "app.py", `
import chainlit as cl
from openai import AsyncOpenAI

client = AsyncOpenAI(base_url="http://localhost:4000/v1", api_key="any")

@cl.on_message
async def main(message: cl.Message):
    response = await client.chat.completions.create(
        model="gpt-4o",
        messages=[{"role": "user", "content": message.content}],
        stream=True,
    )
    msg = cl.Message(content="")
    async for chunk in response:
        if chunk.choices[0].delta.content:
            await msg.stream_token(chunk.choices[0].delta.content)
    await msg.send()`);
  readme("chainlit", "Chainlit", "Chat Platforms", "Config guide",
    "Build Chainlit Python chat UIs with Stockyard as the LLM backend.",
    "1. `pip install chainlit openai`\n2. Copy `app.py`\n3. `chainlit run app.py` — all calls route through Stockyard",
    ["app.py"]);
});

gen("gradio", "Gradio / HuggingFace", "Chat Platforms", "Component", "Use Stockyard with Gradio chat interfaces.", () => {
  write("gradio", "app.py", `
import gradio as gr
from openai import OpenAI

client = OpenAI(base_url="http://localhost:4000/v1", api_key="any")

def chat(message, history):
    messages = [{"role": "user" if i%2==0 else "assistant", "content": m}
                for i, m in enumerate([m for pair in history for m in pair] + [message])]
    r = client.chat.completions.create(model="gpt-4o", messages=messages)
    return r.choices[0].message.content

demo = gr.ChatInterface(chat, title="Chat via Stockyard")
demo.launch()`);
  readme("gradio", "Gradio / HuggingFace", "Chat Platforms", "Component",
    "Build Gradio chat demos with Stockyard backend.",
    "1. `pip install gradio openai`\n2. Copy `app.py`\n3. `python app.py` — Stockyard handles all LLM calls",
    ["app.py"]);
});

gen("streamlit", "Streamlit", "Chat Platforms", "PyPI component", "Streamlit AI apps with Stockyard backend.", () => {
  write("streamlit", "app.py", `
import streamlit as st
from openai import OpenAI

client = OpenAI(base_url="http://localhost:4000/v1", api_key="any")

st.title("Chat via Stockyard")

if prompt := st.chat_input("Ask anything"):
    st.chat_message("user").write(prompt)
    r = client.chat.completions.create(
        model="gpt-4o",
        messages=[{"role": "user", "content": prompt}],
    )
    st.chat_message("assistant").write(r.choices[0].message.content)`);
  readme("streamlit", "Streamlit", "Chat Platforms", "PyPI component",
    "Build Streamlit AI apps with Stockyard backend for cost control.",
    "1. `pip install streamlit openai`\n2. Copy `app.py`\n3. `streamlit run app.py`",
    ["app.py"]);
});

gen("mesop", "Mesop", "Chat Platforms", "Config + example", "Google's Mesop UI framework with Stockyard.", () => {
  write("mesop", "app.py", `
import mesop as me
from openai import OpenAI

client = OpenAI(base_url="http://localhost:4000/v1", api_key="any")

@me.page(path="/")
def page():
    me.text("Chat via Stockyard")
    # Build Mesop chat UI with Stockyard-routed LLM calls`);
  readme("mesop", "Mesop", "Chat Platforms", "Config + example",
    "Use Google's Mesop Python UI framework with Stockyard.",
    "1. `pip install mesop openai`\n2. Point OpenAI client at Stockyard\n3. Build Mesop UI normally",
    ["app.py"]);
});

// ═══════════════════════════════════════════════════════════════
// 3.5 CLOUD & INFRASTRUCTURE (14)
// ═══════════════════════════════════════════════════════════════

gen("aws-lambda", "AWS Lambda", "Cloud", "Lambda layer + SAM", "Run Stockyard as a Lambda layer.", () => {
  write("aws-lambda", "template.yaml", `
AWSTemplateFormatVersion: '2010-09-09'
Transform: AWS::Serverless-2016-10-31

Resources:
  StockyardFunction:
    Type: AWS::Serverless::Function
    Properties:
      Handler: bootstrap
      Runtime: provided.al2023
      CodeUri: ./stockyard-binary/
      MemorySize: 256
      Timeout: 30
      Environment:
        Variables:
          OPENAI_API_KEY: !Ref OpenAIKey
          STOCKYARD_PORT: "4000"
      Events:
        Api:
          Type: Api
          Properties:
            Path: /{proxy+}
            Method: ANY`);
  readme("aws-lambda", "AWS Lambda", "Cloud", "Lambda layer + SAM",
    "Deploy Stockyard as an AWS Lambda function for serverless LLM proxy.",
    "1. Download Stockyard Linux binary\n2. Deploy with SAM template\n3. Point your app at the Lambda URL",
    ["template.yaml"]);
});

gen("aws-cdk", "AWS CDK", "Cloud", "npm package", "Deploy Stockyard with AWS CDK.", () => {
  write("aws-cdk", "stockyard-stack.ts", `
import * as cdk from "aws-cdk-lib";
import * as ecs from "aws-cdk-lib/aws-ecs";
import * as ecsPatterns from "aws-cdk-lib/aws-ecs-patterns";

export class StockyardStack extends cdk.Stack {
  constructor(scope: cdk.App, id: string) {
    super(scope, id);
    new ecsPatterns.ApplicationLoadBalancedFargateService(this, "Stockyard", {
      taskImageOptions: {
        image: ecs.ContainerImage.fromRegistry("stockyard/stockyard:latest"),
        containerPort: 4000,
        environment: {
          OPENAI_API_KEY: process.env.OPENAI_API_KEY!,
        },
      },
      publicLoadBalancer: true,
    });
  }
}`);
  readme("aws-cdk", "AWS CDK", "Cloud", "npm package",
    "Deploy Stockyard on AWS ECS Fargate with CDK. One construct, production-ready.",
    "1. `npm install aws-cdk-lib`\n2. Add `StockyardStack` to your CDK app\n3. `cdk deploy`",
    ["stockyard-stack.ts"]);
});

gen("terraform", "Terraform", "Cloud", "Terraform Registry", "Deploy Stockyard with Terraform.", () => {
  write("terraform", "main.tf", `
# Stockyard on Docker / any cloud
resource "docker_container" "stockyard" {
  name  = "stockyard"
  image = "stockyard/stockyard:latest"

  ports {
    internal = 4000
    external = 4000
  }

  env = [
    "OPENAI_API_KEY=\${var.openai_key}",
  ]

  volumes {
    host_path      = "/opt/stockyard/data"
    container_path = "/data"
  }
}

variable "openai_key" {
  type      = string
  sensitive = true
}`);
  readme("terraform", "Terraform", "Cloud", "Terraform Registry",
    "Deploy Stockyard with Terraform on any cloud provider.",
    "1. Copy `main.tf`\n2. `terraform init && terraform apply`\n3. Stockyard running on port 4000",
    ["main.tf"]);
});

gen("helm", "Kubernetes Helm", "Cloud", "Helm chart", "Deploy Stockyard on Kubernetes.", () => {
  write("helm", "values.yaml", `
replicaCount: 1
image:
  repository: stockyard/stockyard
  tag: latest
  pullPolicy: IfNotPresent
service:
  type: ClusterIP
  port: 4000
env:
  - name: OPENAI_API_KEY
    valueFrom:
      secretKeyRef:
        name: stockyard-secrets
        key: openai-api-key
persistence:
  enabled: true
  size: 1Gi
  storageClass: ""
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 256Mi`);
  write("helm", "Chart.yaml", `
apiVersion: v2
name: stockyard
description: LLM infrastructure proxy — 125 tools in one binary
version: 0.1.0
appVersion: "0.1.0"
type: application`);
  readme("helm", "Kubernetes Helm", "Cloud", "Helm chart",
    "Deploy Stockyard on Kubernetes with Helm.",
    "1. `helm install stockyard ./helm`\n2. Create secret with API keys\n3. Port-forward or expose via ingress",
    ["Chart.yaml", "values.yaml"]);
});

gen("pulumi", "Pulumi", "Cloud", "Multi-language", "Deploy Stockyard with Pulumi.", () => {
  write("pulumi", "index.ts", `
import * as docker from "@pulumi/docker";

const stockyard = new docker.Container("stockyard", {
  image: "stockyard/stockyard:latest",
  ports: [{ internal: 4000, external: 4000 }],
  envs: [\`OPENAI_API_KEY=\${process.env.OPENAI_API_KEY}\`],
});

export const url = stockyard.ports.apply(p => \`http://localhost:\${p![0].external}\`);`);
  readme("pulumi", "Pulumi", "Cloud", "Multi-language",
    "Deploy Stockyard with Pulumi in TypeScript, Python, Go, or C#.",
    "1. `pulumi new typescript`\n2. Copy `index.ts`\n3. `pulumi up`",
    ["index.ts"]);
});

const oneClickPlatforms = [
  ["railway", "Railway", "One-click deploy", "railway.json", `{\n  "build": { "dockerfilePath": "Dockerfile" },\n  "deploy": {\n    "startCommand": "./stockyard",\n    "healthcheckPath": "/health",\n    "restartPolicyType": "ON_FAILURE"\n  }\n}`],
  ["render", "Render", "render.yaml", "render.yaml", `services:\n  - type: web\n    name: stockyard\n    runtime: docker\n    dockerfilePath: ./Dockerfile\n    envVars:\n      - key: OPENAI_API_KEY\n        sync: false\n    healthCheckPath: /health`],
  ["fly-io", "Fly.io", "fly.toml config", "fly.toml", `app = "stockyard"\nprimary_region = "iad"\n\n[build]\n  image = "stockyard/stockyard:latest"\n\n[env]\n  PORT = "4000"\n\n[http_service]\n  internal_port = 4000\n  force_https = true\n  auto_stop_machines = true\n  auto_start_machines = true\n  min_machines_running = 0\n\n[[vm]]\n  cpu_kind = "shared"\n  cpus = 1\n  memory_mb = 256`],
  ["coolify", "Coolify", "Service definition", "docker-compose.yml", `version: "3"\nservices:\n  stockyard:\n    image: stockyard/stockyard:latest\n    ports:\n      - "4000:4000"\n    environment:\n      - OPENAI_API_KEY=\${OPENAI_API_KEY}\n    volumes:\n      - stockyard-data:/data\nvolumes:\n  stockyard-data:`],
  ["caprover", "CapRover", "App definition", "captain-definition", `{\n  "schemaVersion": 2,\n  "imageName": "stockyard/stockyard:latest"\n}`],
  ["portainer", "Portainer", "JSON template", "template.json", `{\n  "version": "2",\n  "templates": [{\n    "type": 1,\n    "title": "Stockyard",\n    "description": "LLM infrastructure proxy — 125 tools",\n    "image": "stockyard/stockyard:latest",\n    "ports": ["4000/tcp"],\n    "env": [{"name": "OPENAI_API_KEY", "label": "OpenAI API Key"}]\n  }]\n}`],
  ["unraid", "Unraid", "XML template", "stockyard.xml", `<?xml version="1.0"?>\n<Container version="2">\n  <Name>Stockyard</Name>\n  <Repository>stockyard/stockyard:latest</Repository>\n  <Network>bridge</Network>\n  <Port>\n    <ContainerPort>4000</ContainerPort>\n    <HostPort>4000</HostPort>\n    <Protocol>tcp</Protocol>\n  </Port>\n  <Config Name="OpenAI Key" Target="OPENAI_API_KEY" Default="" Mode="" Description="Your OpenAI API key" Type="Variable" Display="always" Required="true"/>\n</Container>`],
  ["truenas", "TrueNAS SCALE", "Helm chart", "values.yaml", `image:\n  repository: stockyard/stockyard\n  tag: latest\nservice:\n  main:\n    ports:\n      main:\n        port: 4000\npersistence:\n  data:\n    enabled: true\n    mountPath: /data`],
  ["synology", "Synology Docker", "Compose template", "docker-compose.yml", `version: "3"\nservices:\n  stockyard:\n    image: stockyard/stockyard:latest\n    container_name: stockyard\n    ports:\n      - "4000:4000"\n    environment:\n      - OPENAI_API_KEY=\${OPENAI_API_KEY}\n    volumes:\n      - /volume1/docker/stockyard:/data\n    restart: unless-stopped`],
];

for (const [dir, name, type, file, content] of oneClickPlatforms) {
  gen(dir, name, "Cloud", type, `Deploy Stockyard on ${name}.`, () => {
    write(dir, file, content);
    readme(dir, name, "Cloud", type,
      `Deploy Stockyard on ${name} with a single config file.`,
      `1. Copy \`${file}\` to your project\n2. Deploy to ${name}\n3. Set OPENAI_API_KEY in environment`,
      [file]);
  });
}

// ═══════════════════════════════════════════════════════════════
// 3.6 OBSERVABILITY & MONITORING (9)
// ═══════════════════════════════════════════════════════════════

gen("prometheus", "Prometheus", "Observability", "Built-in /metrics", "Scrape Stockyard metrics with Prometheus.", () => {
  write("prometheus", "prometheus.yml", `
# Add to your prometheus.yml scrape_configs
scrape_configs:
  - job_name: stockyard
    scrape_interval: 15s
    static_configs:
      - targets: ["localhost:4000"]
    metrics_path: /metrics`);
  write("prometheus", "alerts.yml", `
groups:
  - name: stockyard
    rules:
      - alert: HighErrorRate
        expr: rate(stockyard_requests_errors_total[5m]) > 0.1
        for: 2m
        labels: { severity: warning }
        annotations: { summary: "Stockyard error rate above 10%" }
      - alert: HighLatency
        expr: histogram_quantile(0.95, rate(stockyard_request_duration_seconds_bucket[5m])) > 5
        for: 3m
        labels: { severity: warning }
        annotations: { summary: "Stockyard p95 latency above 5s" }
      - alert: BudgetNearLimit
        expr: stockyard_spend_ratio > 0.9
        for: 1m
        labels: { severity: critical }
        annotations: { summary: "Stockyard budget above 90%" }`);
  readme("prometheus", "Prometheus", "Observability", "Built-in /metrics",
    "Scrape Stockyard's built-in Prometheus metrics endpoint for monitoring.",
    "1. Add scrape config to `prometheus.yml`\n2. Import alert rules from `alerts.yml`\n3. Metrics at `http://localhost:4000/metrics`",
    ["prometheus.yml", "alerts.yml"]);
});

gen("grafana", "Grafana", "Observability", "JSON import", "Pre-built Grafana dashboard for Stockyard.", () => {
  write("grafana", "dashboard.json", `
{
  "dashboard": {
    "title": "Stockyard LLM Proxy",
    "panels": [
      { "title": "Requests/sec", "type": "timeseries", "targets": [{"expr": "rate(stockyard_requests_total[5m])"}] },
      { "title": "p95 Latency", "type": "timeseries", "targets": [{"expr": "histogram_quantile(0.95, rate(stockyard_request_duration_seconds_bucket[5m]))"}] },
      { "title": "Cache Hit Rate", "type": "gauge", "targets": [{"expr": "stockyard_cache_hit_rate"}] },
      { "title": "Spend Today ($)", "type": "stat", "targets": [{"expr": "stockyard_spend_today_usd"}] },
      { "title": "Error Rate", "type": "timeseries", "targets": [{"expr": "rate(stockyard_requests_errors_total[5m])"}] },
      { "title": "Tokens/min", "type": "timeseries", "targets": [{"expr": "rate(stockyard_tokens_total[1m]) * 60"}] }
    ]
  }
}`);
  readme("grafana", "Grafana", "Observability", "JSON import",
    "Import a pre-built Grafana dashboard for Stockyard. 30-second setup.",
    "1. Enable Prometheus scraping of Stockyard\n2. Import `dashboard.json` in Grafana\n3. Six panels: RPS, latency, cache, spend, errors, tokens",
    ["dashboard.json"]);
});

gen("opentelemetry", "OpenTelemetry (OTLP)", "Observability", "Built-in exporter", "Export traces to any OTLP-compatible backend.", () => {
  write("opentelemetry", "stockyard.yml", `
# Stockyard OTLP config
telemetry:
  otlp:
    endpoint: "http://localhost:4317"  # OTLP gRPC endpoint
    # or: "http://localhost:4318"      # OTLP HTTP endpoint
    service_name: "stockyard"
    # Works with: Jaeger, Datadog, New Relic, Honeycomb, Grafana Tempo`);
  readme("opentelemetry", "OpenTelemetry (OTLP)", "Observability", "Built-in exporter",
    "Export Stockyard traces to any OTLP-compatible backend: Jaeger, Datadog, New Relic, Honeycomb, Grafana Tempo.",
    "1. Set OTLP endpoint in config\n2. Traces appear in your observability platform\n3. Per-request spans with latency, model, cost",
    ["stockyard.yml"]);
});

gen("datadog", "Datadog", "Observability", "Integration tile", "Send Stockyard metrics to Datadog.", () => {
  write("datadog", "conf.yaml", `
# /etc/datadog-agent/conf.d/stockyard.d/conf.yaml
init_config:

instances:
  - openmetrics_endpoint: http://localhost:4000/metrics
    namespace: stockyard
    metrics:
      - stockyard_requests_total
      - stockyard_request_duration_seconds
      - stockyard_tokens_total
      - stockyard_spend_today_usd
      - stockyard_cache_hit_rate`);
  readme("datadog", "Datadog", "Observability", "Integration tile",
    "Send Stockyard metrics to Datadog via OpenMetrics.",
    "1. Copy `conf.yaml` to Datadog agent config\n2. Restart Datadog agent\n3. Metrics appear as `stockyard.*`",
    ["conf.yaml"]);
});

gen("langfuse", "Langfuse", "Observability", "API exporter", "Export Stockyard traces to Langfuse.", () => {
  write("langfuse", "stockyard.yml", `
# Stockyard + Langfuse integration
telemetry:
  langfuse:
    public_key: "pk-lf-..."
    secret_key: "sk-lf-..."
    host: "https://cloud.langfuse.com"  # or self-hosted URL`);
  readme("langfuse", "Langfuse", "Observability", "API exporter",
    "Export Stockyard request traces to Langfuse for LLM-specific observability.",
    "1. Get Langfuse API keys\n2. Add to Stockyard config\n3. All LLM calls appear in Langfuse",
    ["stockyard.yml"]);
});

gen("langsmith", "Langsmith", "Observability", "Trace exporter", "Feed Stockyard traces to LangSmith.", () => {
  write("langsmith", "setup.md", `
# LangSmith + Stockyard

Set environment variables:
\`\`\`bash
export LANGCHAIN_TRACING_V2=true
export LANGCHAIN_API_KEY=ls-...
export LANGCHAIN_ENDPOINT=https://api.smith.langchain.com
\`\`\`

Stockyard can export traces to LangSmith when using LangChain through the proxy.
For non-LangChain apps, use Stockyard's OTLP export instead.
`);
  readme("langsmith", "Langsmith", "Observability", "Trace exporter",
    "Feed Stockyard traces to LangSmith for LangChain-ecosystem observability.",
    "1. Set LangSmith env vars\n2. Use LangChain through Stockyard\n3. Traces appear in LangSmith automatically",
    ["setup.md"]);
});

gen("helicone", "Helicone", "Observability", "Header + webhook", "Use Stockyard alongside Helicone.", () => {
  write("helicone", "setup.md", `
# Helicone + Stockyard

Stockyard and Helicone can coexist:
- Stockyard: infrastructure (caching, rate limiting, failover, cost caps)
- Helicone: logging and analytics

Option 1: Stockyard proxies to Helicone which proxies to OpenAI
  stockyard.yml providers.openai.base_url: "https://oai.hconeai.com/v1"

Option 2: Stockyard webhook sends logs to Helicone
  stockyard.yml webhooks.log_url: "https://api.helicone.ai/v1/log"
`);
  readme("helicone", "Helicone", "Observability", "Header + webhook",
    "Use Stockyard for infrastructure and Helicone for logging side-by-side.",
    "1. Set Helicone as the upstream provider URL\n2. Or use webhook-based logging\n3. Best of both worlds",
    ["setup.md"]);
});

gen("wandb", "Weights & Biases", "Observability", "API integration", "Log Stockyard metrics to W&B.", () => {
  write("wandb", "setup.md", `
# W&B + Stockyard

Export Stockyard metrics to W&B for experiment tracking:

\`\`\`python
import wandb
import requests

wandb.init(project="llm-proxy")

# Poll Stockyard stats and log to W&B
stats = requests.get("http://localhost:4000/api/stats").json()
wandb.log({
    "requests_total": stats["requests"],
    "cache_hit_rate": stats["cache_hit_rate"],
    "spend_today": stats["spend_today"],
    "avg_latency": stats["avg_latency_ms"],
})
\`\`\`
`);
  readme("wandb", "Weights & Biases", "Observability", "API integration",
    "Log Stockyard metrics to W&B for ML experiment tracking.",
    "1. Poll Stockyard's `/api/stats` endpoint\n2. Log to W&B runs\n3. Track cost/latency/quality across experiments",
    ["setup.md"]);
});

gen("arize", "Arize / Phoenix", "Observability", "OTLP/API export", "Export to Arize for LLM observability.", () => {
  write("arize", "setup.md", `
# Arize / Phoenix + Stockyard

Use Stockyard's OTLP export to send traces to Arize:

\`\`\`yaml
telemetry:
  otlp:
    endpoint: "https://otlp.arize.com"
    headers:
      space_key: "your-space-key"
      api_key: "your-api-key"
\`\`\`

For Phoenix (open-source):
\`\`\`yaml
telemetry:
  otlp:
    endpoint: "http://localhost:6006"
\`\`\`
`);
  readme("arize", "Arize / Phoenix", "Observability", "OTLP/API export",
    "Export Stockyard traces to Arize or Phoenix for LLM observability.",
    "1. Set OTLP endpoint to Arize or Phoenix\n2. Traces flow automatically\n3. LLM-specific dashboards in Arize/Phoenix",
    ["setup.md"]);
});

// ═══════════════════════════════════════════════════════════════
// 3.7 AGENT FRAMEWORKS (8)
// ═══════════════════════════════════════════════════════════════

gen("openai-swarm", "OpenAI Swarm", "Agent Frameworks", "Config guide", "Route OpenAI Swarm agents through Stockyard.", () => {
  write("openai-swarm", "example.py", `
from openai import OpenAI
from swarm import Swarm

# Point Swarm at Stockyard
client = OpenAI(base_url="http://localhost:4000/v1", api_key="any")
swarm = Swarm(client=client)

# All agent handoffs and tool calls route through Stockyard
# AgentGuard recommended for session-level cost caps`);
  readme("openai-swarm", "OpenAI Swarm", "Agent Frameworks", "Config guide",
    "Route OpenAI Swarm multi-agent orchestration through Stockyard. AgentGuard recommended for session limits.",
    "1. Pass custom OpenAI client to Swarm\n2. Enable AgentGuard for per-session cost caps\n3. All agent calls tracked",
    ["example.py"]);
});

// autogen already exists, skip it

gen("metagpt", "MetaGPT", "Agent Frameworks", "Provider config", "Route MetaGPT through Stockyard.", () => {
  write("metagpt", "config.yaml", `
# ~/.metagpt/config.yaml
llm:
  api_type: openai
  base_url: http://localhost:4000/v1
  api_key: any-string
  model: gpt-4o`);
  readme("metagpt", "MetaGPT", "Agent Frameworks", "Provider config",
    "Route MetaGPT software dev agents through Stockyard. CostCap essential — MetaGPT generates entire codebases.",
    "1. Set `base_url` in MetaGPT config\n2. Enable CostCap with aggressive limits\n3. IdleKill recommended for runaway agents",
    ["config.yaml"]);
});

gen("babyagi", "BabyAGI / AutoGPT", "Agent Frameworks", "Config + safety", "Safety-first Stockyard setup for autonomous agents.", () => {
  write("babyagi", ".env", `
# BabyAGI / AutoGPT + Stockyard
# CRITICAL: These agents can burn hundreds of dollars
OPENAI_API_BASE=http://localhost:4000/v1
OPENAI_API_KEY=any-string

# Recommended Stockyard config:
# costcap.daily: 10.00    # Hard daily limit
# idlekill.max_duration: 60s  # Kill requests over 60s
# agentguard.max_calls: 50    # Max 50 calls per session`);
  readme("babyagi", "BabyAGI / AutoGPT", "Agent Frameworks", "Config + safety guide",
    "Run autonomous agents through Stockyard with safety rails. CostCap + IdleKill + AgentGuard are essential.",
    "1. Set `OPENAI_API_BASE` to Stockyard\n2. Enable CostCap with tight daily limits\n3. Enable IdleKill and AgentGuard\n4. Monitor dashboard closely",
    [".env"]);
});

gen("pydantic-ai", "Pydantic AI", "Agent Frameworks", "Model provider", "Type-safe AI with Stockyard.", () => {
  write("pydantic-ai", "example.py", `
from pydantic_ai import Agent
from pydantic_ai.models.openai import OpenAIModel

model = OpenAIModel(
    "gpt-4o",
    base_url="http://localhost:4000/v1",
    api_key="any-string",
)

agent = Agent(model, system_prompt="You are helpful.")`);
  readme("pydantic-ai", "Pydantic AI", "Agent Frameworks", "Model provider",
    "Use Pydantic AI type-safe agents through Stockyard.",
    "1. Set `base_url` when creating OpenAIModel\n2. All agent calls route through Stockyard",
    ["example.py"]);
});

gen("mastra", "Mastra", "Agent Frameworks", "Config guide", "Route Mastra TS agents through Stockyard.", () => {
  write("mastra", "example.ts", `
import { Agent } from "@mastra/core";
import OpenAI from "openai";

const client = new OpenAI({
  baseURL: "http://localhost:4000/v1",
  apiKey: "any-string",
});

// Use client with Mastra agents`);
  readme("mastra", "Mastra", "Agent Frameworks", "Config guide",
    "Route Mastra TypeScript-first agents through Stockyard.",
    "1. Create OpenAI client pointing at Stockyard\n2. Pass to Mastra agents",
    ["example.ts"]);
});

gen("composio", "Composio", "Agent Frameworks", "Config guide", "Use Composio tool platform with Stockyard.", () => {
  write("composio", "setup.md", `
# Composio + Stockyard

Composio provides 150+ tool integrations for AI agents.
Route the LLM calls through Stockyard:

\`\`\`python
from openai import OpenAI
from composio_openai import ComposioToolSet

client = OpenAI(base_url="http://localhost:4000/v1", api_key="any")
toolset = ComposioToolSet()
\`\`\`

Stockyard handles cost/caching/routing. Composio handles tools.
`);
  readme("composio", "Composio", "Agent Frameworks", "Config guide",
    "Use Composio's 150+ tool integrations with Stockyard-routed LLM calls.",
    "1. Point OpenAI client at Stockyard\n2. Use Composio ToolSet normally\n3. LLM calls go through Stockyard, tool calls go through Composio",
    ["setup.md"]);
});

gen("julep", "Julep", "Agent Frameworks", "Provider config", "Route Julep stateful agents through Stockyard.", () => {
  write("julep", "setup.md", `
# Julep + Stockyard

Configure Julep to use Stockyard as the LLM provider:

\`\`\`yaml
# julep config
llm:
  provider: openai
  base_url: http://stockyard:4000/v1
  api_key: any-string
\`\`\`
`);
  readme("julep", "Julep", "Agent Frameworks", "Provider config",
    "Route Julep stateful AI agents through Stockyard.",
    "1. Set `base_url` in Julep config\n2. Agent sessions tracked by Stockyard",
    ["setup.md"]);
});

// ═══════════════════════════════════════════════════════════════
// 3.8 LOW-CODE / NO-CODE (5)
// ═══════════════════════════════════════════════════════════════

gen("bubble", "Bubble.io", "Low-Code", "Bubble plugin", "Connect Bubble.io apps to Stockyard.", () => {
  write("bubble", "setup.md", `
# Bubble.io + Stockyard

1. Use the API Connector plugin in Bubble
2. Add new API:
   - Name: Stockyard
   - Base URL: http://your-stockyard-host:4000/v1
   - Authentication: None (Stockyard handles it)
3. Add call: POST /chat/completions
4. Body: {"model":"gpt-4o","messages":[{"role":"user","content":"<prompt>"}]}
`);
  readme("bubble", "Bubble.io", "Low-Code", "Bubble plugin",
    "Connect Bubble.io no-code apps to Stockyard for cost-controlled LLM features.",
    "1. Deploy Stockyard publicly\n2. Use API Connector in Bubble\n3. Point at Stockyard URL",
    ["setup.md"]);
});

gen("retool", "Retool", "Low-Code", "REST API resource", "Use Stockyard in Retool internal tools.", () => {
  write("retool", "setup.md", `
# Retool + Stockyard

1. Resources > Add Resource > REST API
2. Base URL: http://your-stockyard-host:4000/v1
3. Create query: POST /chat/completions
4. Body: {"model":"gpt-4o","messages":[{"role":"user","content":{{textInput.value}}}]}
`);
  readme("retool", "Retool", "Low-Code", "REST API resource",
    "Add Stockyard as a REST resource in Retool for internal AI tools with cost tracking.",
    "1. Add REST API resource in Retool\n2. Point at Stockyard\n3. Cost tracking for enterprise internal tools",
    ["setup.md"]);
});

gen("supabase", "Supabase Edge Functions", "Low-Code", "Function template", "Call LLMs from Supabase Edge Functions via Stockyard.", () => {
  write("supabase", "stockyard-chat.ts", `
// supabase/functions/stockyard-chat/index.ts
import { serve } from "https://deno.land/std/http/server.ts";

serve(async (req) => {
  const { prompt } = await req.json();
  const response = await fetch("http://your-stockyard-host:4000/v1/chat/completions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      model: "gpt-4o",
      messages: [{ role: "user", content: prompt }],
    }),
  });
  const data = await response.json();
  return new Response(JSON.stringify({ reply: data.choices[0].message.content }));
});`);
  readme("supabase", "Supabase Edge Functions", "Low-Code", "Function template",
    "Call LLMs from Supabase Edge Functions through Stockyard.",
    "1. Deploy Stockyard\n2. Copy edge function template\n3. `supabase functions deploy stockyard-chat`",
    ["stockyard-chat.ts"]);
});

gen("val-town", "Val Town", "Low-Code", "Val template", "Serverless LLM calls through Stockyard.", () => {
  write("val-town", "example.ts", `
// Val Town val
import { OpenAI } from "npm:openai";

const client = new OpenAI({
  baseURL: "http://your-stockyard-host:4000/v1",
  apiKey: "any-string",
});

export async function chat(prompt: string) {
  const r = await client.chat.completions.create({
    model: "gpt-4o",
    messages: [{ role: "user", content: prompt }],
  });
  return r.choices[0].message.content;
}`);
  readme("val-town", "Val Town", "Low-Code", "Val template",
    "Serverless LLM functions in Val Town via Stockyard.",
    "1. Deploy Stockyard publicly\n2. Import OpenAI in your val\n3. Point at Stockyard URL",
    ["example.ts"]);
});

gen("replit", "Replit", "Low-Code", "Template + Nix", "Run Stockyard on Replit.", () => {
  write("replit", ".replit", `
run = "./stockyard"
entrypoint = "main.go"

[nix]
channel = "stable-24_05"

[env]
OPENAI_API_KEY = ""

[[ports]]
localPort = 4000
externalPort = 80`);
  readme("replit", "Replit", "Low-Code", "Template + Nix",
    "Run Stockyard as a Replit project for instant prototyping.",
    "1. Fork the Replit template\n2. Set OPENAI_API_KEY in Secrets\n3. Click Run",
    [".replit"]);
});

// ═══════════════════════════════════════════════════════════════
// 3.9 PACKAGE REGISTRIES (9)
// ═══════════════════════════════════════════════════════════════

gen("pypi", "PyPI", "Package Registries", "Python SDK", "pip install stockyard.", () => {
  write("pypi", "setup.py", `
from setuptools import setup

setup(
    name="stockyard",
    version="0.1.0",
    description="LLM infrastructure proxy — 125 tools in one binary",
    author="Stockyard",
    url="https://stockyard.dev",
    py_modules=["stockyard"],
    entry_points={"console_scripts": ["stockyard=stockyard:main"]},
    python_requires=">=3.8",
)`);
  readme("pypi", "PyPI", "Package Registries", "Python SDK",
    "Install Stockyard via pip. Downloads the Go binary for your platform.",
    "1. `pip install stockyard`\n2. `stockyard --config stockyard.yml`",
    ["setup.py"]);
});

gen("cargo", "Cargo (Rust)", "Package Registries", "Crate + binary", "cargo install stockyard.", () => {
  write("cargo", "setup.md", `
# Cargo doesn't natively distribute non-Rust binaries.
# Use cargo-binstall for binary distribution:
cargo binstall stockyard

# Or download directly:
curl -fsSL https://stockyard.dev/install.sh | sh
`);
  readme("cargo", "Cargo (Rust)", "Package Registries", "Crate + binary",
    "Install Stockyard via cargo-binstall.",
    "1. `cargo binstall stockyard`\n2. Or use the install script",
    ["setup.md"]);
});

gen("nuget", "NuGet (.NET)", "Package Registries", "C# SDK", "dotnet add package Stockyard.", () => {
  write("nuget", "setup.md", `
# NuGet SDK wraps Stockyard binary
dotnet add package Stockyard

# Usage in C#:
# using Stockyard;
# var proxy = new StockyardProxy(config);
# proxy.Start();
`);
  readme("nuget", "NuGet (.NET)", "Package Registries", "C# SDK",
    "Install Stockyard .NET SDK via NuGet.",
    "1. `dotnet add package Stockyard`\n2. Start proxy from your .NET app",
    ["setup.md"]);
});

const pkgManagers = [
  ["scoop", "Scoop", "Windows manifest", "stockyard.json", `{\n  "version": "0.1.0",\n  "description": "LLM infrastructure proxy",\n  "homepage": "https://stockyard.dev",\n  "license": "MIT",\n  "architecture": {\n    "64bit": { "url": "https://github.com/stockyard-dev/stockyard/releases/download/v0.1.0/stockyard_windows_amd64.zip" },\n    "arm64": { "url": "https://github.com/stockyard-dev/stockyard/releases/download/v0.1.0/stockyard_windows_arm64.zip" }\n  },\n  "bin": "stockyard.exe"\n}`],
  ["chocolatey", "Chocolatey", "Windows package", "stockyard.nuspec", `<?xml version="1.0"?>\n<package xmlns="http://schemas.microsoft.com/packaging/2015/06/nuspec.xsd">\n  <metadata>\n    <id>stockyard</id>\n    <version>0.1.0</version>\n    <title>Stockyard</title>\n    <authors>Stockyard</authors>\n    <description>LLM infrastructure proxy — 125 tools in one binary</description>\n    <projectUrl>https://stockyard.dev</projectUrl>\n    <tags>llm proxy ai openai</tags>\n  </metadata>\n</package>`],
  ["snap", "Snap", "Snapcraft config", "snapcraft.yaml", `name: stockyard\nversion: '0.1.0'\nsummary: LLM infrastructure proxy\ndescription: 125 LLM tools in one binary.\nbase: core22\nconfinement: strict\napps:\n  stockyard:\n    command: stockyard\n    plugs: [network, network-bind]\nparts:\n  stockyard:\n    plugin: dump\n    source: https://github.com/stockyard-dev/stockyard/releases/download/v0.1.0/stockyard_linux_amd64.tar.gz`],
  ["winget", "WinGet", "Microsoft manifest", "Stockyard.Stockyard.yaml", `PackageIdentifier: Stockyard.Stockyard\nPackageVersion: 0.1.0\nPackageName: Stockyard\nPublisher: Stockyard\nLicense: MIT\nShortDescription: LLM infrastructure proxy — 125 tools in one binary\nInstallers:\n  - Architecture: x64\n    InstallerUrl: https://github.com/stockyard-dev/stockyard/releases/download/v0.1.0/stockyard_windows_amd64.zip\n    InstallerType: zip\nManifestType: singleton\nManifestVersion: 1.4.0`],
  ["aur", "AUR (Arch)", "PKGBUILD", "PKGBUILD", `pkgname=stockyard\npkgver=0.1.0\npkgrel=1\npkgdesc="LLM infrastructure proxy — 125 tools in one binary"\narch=('x86_64' 'aarch64')\nurl="https://stockyard.dev"\nlicense=('MIT')\nsource_x86_64=("https://github.com/stockyard-dev/stockyard/releases/download/v\${pkgver}/stockyard_linux_amd64.tar.gz")\nsource_aarch64=("https://github.com/stockyard-dev/stockyard/releases/download/v\${pkgver}/stockyard_linux_arm64.tar.gz")\n\npackage() {\n  install -Dm755 stockyard "\${pkgdir}/usr/bin/stockyard"\n}`],
  ["nix", "Nix", "Nix derivation", "default.nix", `{ lib, stdenv, fetchurl }:\n\nstdenv.mkDerivation rec {\n  pname = "stockyard";\n  version = "0.1.0";\n\n  src = fetchurl {\n    url = "https://github.com/stockyard-dev/stockyard/releases/download/v\${version}/stockyard_linux_amd64.tar.gz";\n    sha256 = "0000000000000000000000000000000000000000000000000000";\n  };\n\n  installPhase = ''\n    install -Dm755 stockyard $out/bin/stockyard\n  '';\n\n  meta = with lib; {\n    description = "LLM infrastructure proxy — 125 tools in one binary";\n    homepage = "https://stockyard.dev";\n    license = licenses.mit;\n  };\n}`],
];

for (const [dir, name, type, file, content] of pkgManagers) {
  gen(dir, name, "Package Registries", type, `Install Stockyard via ${name}.`, () => {
    write(dir, file, content);
    readme(dir, name, "Package Registries", type,
      `Install Stockyard via ${name}.`,
      `1. See \`${file}\` for package manifest\n2. Submit to ${name} registry`,
      [file]);
  });
}

// ═══════════════════════════════════════════════════════════════
// 3.10 VECTOR DB & RAG (5)
// ═══════════════════════════════════════════════════════════════

gen("chroma", "Chroma", "Vector DB", "Python embedding fn", "Cache Chroma embedding calls through Stockyard.", () => {
  write("chroma", "example.py", `
import chromadb
from openai import OpenAI

client = OpenAI(base_url="http://localhost:4000/v1", api_key="any")

# Custom embedding function using Stockyard-cached embeddings
class StockyardEmbedding(chromadb.EmbeddingFunction):
    def __call__(self, input):
        r = client.embeddings.create(model="text-embedding-3-small", input=input)
        return [e.embedding for e in r.data]

collection = chromadb.Client().create_collection("docs", embedding_function=StockyardEmbedding())`);
  readme("chroma", "Chroma", "Vector DB", "Python embedding fn",
    "Cache Chroma embedding calls through Stockyard. EmbedCache provides 100% cache hit rate for re-indexed documents.",
    "1. Start Stockyard with EmbedCache enabled\n2. Use custom embedding function pointing at Stockyard\n3. Re-indexing is free after first run",
    ["example.py"]);
});

gen("weaviate", "Weaviate", "Vector DB", "Vectorizer module", "Route Weaviate embeddings through Stockyard.", () => {
  write("weaviate", "schema.json", `
{
  "class": "Document",
  "vectorizer": "text2vec-openai",
  "moduleConfig": {
    "text2vec-openai": {
      "baseURL": "http://stockyard:4000/v1",
      "model": "text-embedding-3-small"
    }
  }
}`);
  readme("weaviate", "Weaviate", "Vector DB", "Vectorizer module",
    "Route Weaviate embedding vectorization through Stockyard for caching.",
    "1. Set `baseURL` in Weaviate vectorizer config\n2. All embeddings cached by Stockyard",
    ["schema.json"]);
});

gen("qdrant", "Qdrant", "Vector DB", "Config guide", "Cache Qdrant embeddings through Stockyard.", () => {
  write("qdrant", "example.py", `
from qdrant_client import QdrantClient
from openai import OpenAI

embed_client = OpenAI(base_url="http://localhost:4000/v1", api_key="any")

def embed(texts):
    r = embed_client.embeddings.create(model="text-embedding-3-small", input=texts)
    return [e.embedding for e in r.data]

# Use embed() function when upserting to Qdrant`);
  readme("qdrant", "Qdrant", "Vector DB", "Config guide",
    "Cache Qdrant embedding calls through Stockyard.",
    "1. Use OpenAI SDK pointed at Stockyard for embedding calls\n2. Pass vectors to Qdrant client",
    ["example.py"]);
});

gen("pinecone", "Pinecone", "Vector DB", "Client wrapper", "Cache Pinecone embedding calls through Stockyard.", () => {
  write("pinecone", "example.py", `
from pinecone import Pinecone
from openai import OpenAI

embed_client = OpenAI(base_url="http://localhost:4000/v1", api_key="any")

def embed(texts):
    r = embed_client.embeddings.create(model="text-embedding-3-small", input=texts)
    return [e.embedding for e in r.data]

# Cache embeddings through Stockyard, store vectors in Pinecone
pc = Pinecone(api_key="your-pinecone-key")
index = pc.Index("my-index")`);
  readme("pinecone", "Pinecone", "Vector DB", "Client wrapper",
    "Cache embedding calls through Stockyard before storing in Pinecone.",
    "1. Generate embeddings through Stockyard (cached)\n2. Store in Pinecone normally",
    ["example.py"]);
});

gen("pgvector", "pgvector / Supabase", "Vector DB", "Setup guide", "Cache embeddings for pgvector through Stockyard.", () => {
  write("pgvector", "example.py", `
from openai import OpenAI
import psycopg2

embed_client = OpenAI(base_url="http://localhost:4000/v1", api_key="any")

def embed_and_store(text, conn):
    r = embed_client.embeddings.create(model="text-embedding-3-small", input=[text])
    vector = r.data[0].embedding
    cur = conn.cursor()
    cur.execute("INSERT INTO docs (content, embedding) VALUES (%s, %s)", (text, vector))
    conn.commit()`);
  readme("pgvector", "pgvector / Supabase", "Vector DB", "Setup guide",
    "Cache PostgreSQL pgvector embedding calls through Stockyard.",
    "1. Generate embeddings through Stockyard\n2. Store in pgvector table\n3. Re-processing is instant thanks to EmbedCache",
    ["example.py"]);
});

// ═══════════════════════════════════════════════════════════════
// 3.11 CI/CD (3)
// ═══════════════════════════════════════════════════════════════

gen("github-actions", "GitHub Actions", "CI/CD", "Actions Marketplace", "LLM testing in CI with Stockyard.", () => {
  write("github-actions", "stockyard-test.yml", `
name: LLM Tests
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Start Stockyard MockLLM
        run: |
          curl -fsSL https://stockyard.dev/install.sh | sh
          stockyard --product mockllm --config test/mockllm.yml &
          sleep 2
      - name: Run tests
        env:
          OPENAI_API_BASE: http://localhost:4000/v1
          OPENAI_API_KEY: test-key
        run: npm test`);
  readme("github-actions", "GitHub Actions", "CI/CD", "Actions Marketplace",
    "Use Stockyard MockLLM in GitHub Actions for deterministic, free LLM tests in CI.",
    "1. Add workflow file\n2. MockLLM provides deterministic responses\n3. No API keys needed, no API costs",
    ["stockyard-test.yml"]);
});

gen("gitlab-ci", "GitLab CI", "CI/CD", "Template YAML", "LLM testing in GitLab CI.", () => {
  write("gitlab-ci", ".gitlab-ci.yml", `
llm-tests:
  image: ubuntu:24.04
  before_script:
    - curl -fsSL https://stockyard.dev/install.sh | sh
    - stockyard --product mockllm --config test/mockllm.yml &
    - sleep 2
  script:
    - OPENAI_API_BASE=http://localhost:4000/v1 npm test`);
  readme("gitlab-ci", "GitLab CI", "CI/CD", "Template YAML",
    "Use Stockyard MockLLM in GitLab CI pipelines.",
    "1. Add `.gitlab-ci.yml` job\n2. MockLLM runs alongside your tests\n3. Free, deterministic LLM responses",
    [".gitlab-ci.yml"]);
});

gen("circleci", "CircleCI", "CI/CD", "Orb", "LLM testing in CircleCI.", () => {
  write("circleci", "config.yml", `
version: 2.1
jobs:
  llm-tests:
    docker:
      - image: cimg/node:20.0
    steps:
      - checkout
      - run:
          name: Start Stockyard MockLLM
          command: |
            curl -fsSL https://stockyard.dev/install.sh | sh
            stockyard --product mockllm &
          background: true
      - run:
          name: Wait for proxy
          command: sleep 3
      - run:
          name: Run tests
          environment:
            OPENAI_API_BASE: http://localhost:4000/v1
          command: npm test`);
  readme("circleci", "CircleCI", "CI/CD", "Orb",
    "Use Stockyard MockLLM in CircleCI for deterministic LLM tests.",
    "1. Add job to `.circleci/config.yml`\n2. MockLLM starts as background process\n3. Tests run against deterministic responses",
    ["config.yml"]);
});

// ═══════════════════════════════════════════════════════════════
// 3.12 LOCAL LLM & MODEL SERVING (6)
// ═══════════════════════════════════════════════════════════════

gen("lm-studio", "LM Studio", "Local LLM", "Config template", "Use Stockyard in front of LM Studio.", () => {
  write("lm-studio", "stockyard.yml", `
# Stockyard config for LM Studio
providers:
  lmstudio:
    base_url: http://localhost:1234/v1
    api_key: not-needed

# Stockyard adds caching, logging, and cost tracking to local models`);
  readme("lm-studio", "LM Studio", "Local LLM", "Config template",
    "Put Stockyard in front of LM Studio for caching and analytics on local models.",
    "1. Start LM Studio on port 1234\n2. Start Stockyard pointing at LM Studio\n3. Apps connect to Stockyard on port 4000",
    ["stockyard.yml"]);
});

gen("jan-ai", "Jan.ai", "Local LLM", "Provider config", "Route Jan.ai through Stockyard.", () => {
  write("jan-ai", "stockyard.yml", `
providers:
  jan:
    base_url: http://localhost:1337/v1
    api_key: not-needed`);
  readme("jan-ai", "Jan.ai", "Local LLM", "Provider config",
    "Route Jan.ai local and remote LLM calls through Stockyard.",
    "1. Start Jan on default port\n2. Configure Stockyard to proxy Jan\n3. LocalSync can auto-failover to cloud",
    ["stockyard.yml"]);
});

gen("gpt4all", "GPT4All", "Local LLM", "Backend config", "Use Stockyard with GPT4All.", () => {
  write("gpt4all", "stockyard.yml", `
providers:
  gpt4all:
    base_url: http://localhost:4891/v1
    api_key: not-needed`);
  readme("gpt4all", "GPT4All", "Local LLM", "Backend config",
    "Route GPT4All local model calls through Stockyard for caching and logging.",
    "1. Start GPT4All API server\n2. Configure Stockyard to proxy it\n3. LocalSync recommended for cloud failover",
    ["stockyard.yml"]);
});

gen("text-gen-webui", "text-generation-webui", "Local LLM", "Extension/config", "Use Stockyard with oobabooga text-gen-webui.", () => {
  write("text-gen-webui", "stockyard.yml", `
providers:
  textgen:
    base_url: http://localhost:5000/v1
    api_key: not-needed`);
  readme("text-gen-webui", "text-generation-webui", "Local LLM", "Extension/config",
    "Route text-generation-webui calls through Stockyard.",
    "1. Enable OpenAI API in text-gen-webui\n2. Point Stockyard at localhost:5000\n3. All calls cached and logged",
    ["stockyard.yml"]);
});

gen("tabby", "Tabby", "Local LLM", "Model config", "Use Stockyard with Tabby self-hosted Copilot.", () => {
  write("tabby", "setup.md", `
# Tabby + Stockyard

Configure Tabby to route completions through Stockyard:

In Tabby config, set the model endpoint to Stockyard:
  http://localhost:4000/v1

Stockyard proxies to your actual model server (Ollama, vLLM, etc.)
`);
  readme("tabby", "Tabby", "Local LLM", "Model config",
    "Route Tabby self-hosted Copilot through Stockyard for analytics.",
    "1. Configure Tabby model endpoint to Stockyard\n2. Stockyard proxies to your model server\n3. All completions logged and cached",
    ["setup.md"]);
});

gen("mlx-llamacpp", "MLX / llama.cpp", "Local LLM", "Docker Compose", "Sidecar Stockyard with local model servers.", () => {
  write("mlx-llamacpp", "docker-compose.yml", `
version: "3"
services:
  llamacpp:
    image: ghcr.io/ggerganov/llama.cpp:server
    ports:
      - "8080:8080"
    volumes:
      - ./models:/models
    command: -m /models/model.gguf --port 8080

  stockyard:
    image: stockyard/stockyard:latest
    ports:
      - "4000:4000"
    environment:
      - STOCKYARD_PROVIDERS_LOCAL_BASE_URL=http://llamacpp:8080/v1
    depends_on:
      - llamacpp`);
  readme("mlx-llamacpp", "MLX / llama.cpp", "Local LLM", "Docker Compose",
    "Run Stockyard alongside llama.cpp or MLX model servers.",
    "1. `docker compose up`\n2. llama.cpp serves the model, Stockyard adds infrastructure\n3. Apps connect to Stockyard on port 4000",
    ["docker-compose.yml"]);
});

// ═══════════════════════════════════════════════════════════════
// 3.13 API DOCS & TESTING (4)
// ═══════════════════════════════════════════════════════════════

gen("mintlify", "Mintlify Docs", "API Docs", "OpenAPI spec", "Interactive Stockyard API docs.", () => {
  write("mintlify", "mint.json", `
{
  "name": "Stockyard",
  "logo": { "dark": "/logo-dark.svg", "light": "/logo-light.svg" },
  "api": {
    "baseUrl": "http://localhost:4000",
    "auth": { "method": "bearer" }
  },
  "navigation": [
    { "group": "Getting Started", "pages": ["quickstart", "configuration"] },
    { "group": "API Reference", "pages": ["api/chat-completions", "api/embeddings", "api/management"] }
  ]
}`);
  readme("mintlify", "Mintlify Docs", "API Docs", "OpenAPI spec",
    "Interactive API documentation for Stockyard powered by Mintlify.",
    "1. Copy `mint.json` to docs repo\n2. Add OpenAPI spec\n3. Deploy with Mintlify",
    ["mint.json"]);
});

gen("postman", "Postman", "API Docs", "Public workspace", "Explore Stockyard API in Postman.", () => {
  write("postman", "stockyard.postman_collection.json", `
{
  "info": { "name": "Stockyard API", "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json" },
  "item": [
    {
      "name": "Chat Completion",
      "request": {
        "method": "POST",
        "url": "http://localhost:4000/v1/chat/completions",
        "header": [{ "key": "Content-Type", "value": "application/json" }],
        "body": { "mode": "raw", "raw": "{\"model\":\"gpt-4o\",\"messages\":[{\"role\":\"user\",\"content\":\"Hello\"}]}" }
      }
    },
    {
      "name": "Health Check",
      "request": { "method": "GET", "url": "http://localhost:4000/health" }
    },
    {
      "name": "Get Stats",
      "request": { "method": "GET", "url": "http://localhost:4000/api/stats" }
    }
  ]
}`);
  readme("postman", "Postman", "API Docs", "Public workspace",
    "Explore and test the Stockyard API in Postman.",
    "1. Import `stockyard.postman_collection.json`\n2. Start Stockyard on localhost:4000\n3. Send requests from Postman",
    ["stockyard.postman_collection.json"]);
});

gen("insomnia", "Insomnia", "API Docs", "Workspace export", "Explore Stockyard API in Insomnia.", () => {
  write("insomnia", "stockyard-insomnia.json", `
{
  "_type": "export",
  "resources": [
    {
      "_type": "request",
      "name": "Chat Completion",
      "method": "POST",
      "url": "http://localhost:4000/v1/chat/completions",
      "body": { "mimeType": "application/json", "text": "{\"model\":\"gpt-4o\",\"messages\":[{\"role\":\"user\",\"content\":\"Hello\"}]}" }
    },
    {
      "_type": "request",
      "name": "Health Check",
      "method": "GET",
      "url": "http://localhost:4000/health"
    }
  ]
}`);
  readme("insomnia", "Insomnia", "API Docs", "Workspace export",
    "Explore Stockyard API in Insomnia.",
    "1. Import workspace file\n2. Start Stockyard\n3. Test API endpoints",
    ["stockyard-insomnia.json"]);
});

gen("bruno", "Bruno", "API Docs", "Git-native files", "Explore Stockyard API in Bruno.", () => {
  write("bruno", "chat-completion.bru", `
meta {
  name: Chat Completion
  type: http
  seq: 1
}

post {
  url: http://localhost:4000/v1/chat/completions
  body: json
}

body:json {
  {
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello"}]
  }
}`);
  write("bruno", "health.bru", `
meta {
  name: Health Check
  type: http
  seq: 2
}

get {
  url: http://localhost:4000/health
}`);
  readme("bruno", "Bruno", "API Docs", "Git-native files",
    "Explore Stockyard API in Bruno (git-native API client).",
    "1. Open collection folder in Bruno\n2. Start Stockyard\n3. Send requests",
    ["chat-completion.bru", "health.bru"]);
});

// ═══════════════════════════════════════════════════════════════
// 3.14 LLM PROVIDERS (13)
// ═══════════════════════════════════════════════════════════════

const providers = [
  ["openrouter", "OpenRouter", "Config route or replace. Stockyard can sit in front of OpenRouter or replace it entirely.", "stockyard.yml", `providers:\n  openrouter:\n    base_url: https://openrouter.ai/api/v1\n    api_key: \${OPENROUTER_API_KEY}\n\n# Or replace OpenRouter entirely:\n# Stockyard routes to providers directly with failover`],
  ["cloudflare-ai", "Cloudflare AI Gateway", "Migration guide for users outgrowing Cloudflare AI Gateway.", "migration-guide.md", `# Migrating from Cloudflare AI Gateway to Stockyard\n\n## Why?\n- Cloudflare: limited to caching + rate limiting\n- Stockyard: 125 middleware products\n- Self-hosted: your data stays local\n\n## Step 1: Replace gateway URL\nBefore: https://gateway.ai.cloudflare.com/v1/{account}/{gateway}\nAfter:  http://your-stockyard:4000/v1\n\n## Step 2: Move provider keys to Stockyard config\nYour application code doesn't change.`],
  ["bedrock", "Amazon Bedrock", "Route AWS Bedrock models through Stockyard with OpenAI-compatible API.", "stockyard.yml", `providers:\n  bedrock:\n    type: bedrock\n    region: us-east-1\n    access_key: \${AWS_ACCESS_KEY_ID}\n    secret_key: \${AWS_SECRET_ACCESS_KEY}\n    # Models: anthropic.claude-3-sonnet, amazon.titan-text, meta.llama3`],
  ["azure-openai", "Azure OpenAI", "Route Azure OpenAI deployments through Stockyard.", "stockyard.yml", `providers:\n  azure:\n    type: azure\n    endpoint: https://your-resource.openai.azure.com\n    api_key: \${AZURE_OPENAI_KEY}\n    api_version: "2024-02-15-preview"\n    deployment_map:\n      gpt-4o: your-gpt4o-deployment\n      gpt-4o-mini: your-mini-deployment`],
  ["vertex-ai", "Google Vertex AI", "Route Vertex AI through Stockyard with service account auth.", "stockyard.yml", `providers:\n  vertex:\n    type: vertex\n    project: your-gcp-project\n    location: us-central1\n    credentials: \${GOOGLE_APPLICATION_CREDENTIALS}\n    # Models: gemini-1.5-pro, gemini-1.5-flash`],
  ["replicate", "Replicate", "Route Replicate open-source models through Stockyard.", "stockyard.yml", `providers:\n  replicate:\n    type: replicate\n    api_key: \${REPLICATE_API_TOKEN}\n    # Models: meta/llama-3-70b, mistralai/mixtral-8x7b`],
  ["together", "Together AI", "Route Together AI fast inference through Stockyard.", "stockyard.yml", `providers:\n  together:\n    base_url: https://api.together.xyz/v1\n    api_key: \${TOGETHER_API_KEY}\n    # OpenAI-compatible — works out of the box`],
  ["fireworks", "Fireworks AI", "Route Fireworks AI through Stockyard.", "stockyard.yml", `providers:\n  fireworks:\n    base_url: https://api.fireworks.ai/inference/v1\n    api_key: \${FIREWORKS_API_KEY}\n    # OpenAI-compatible`],
  ["perplexity", "Perplexity API", "Route Perplexity search-augmented responses through Stockyard.", "stockyard.yml", `providers:\n  perplexity:\n    base_url: https://api.perplexity.ai\n    api_key: \${PERPLEXITY_API_KEY}\n    # Models: llama-3.1-sonar-small-128k-online`],
  ["mistral", "Mistral API", "Route Mistral AI through Stockyard.", "stockyard.yml", `providers:\n  mistral:\n    base_url: https://api.mistral.ai/v1\n    api_key: \${MISTRAL_API_KEY}\n    # OpenAI-compatible`],
  ["cohere", "Cohere", "Route Cohere embeddings and completions through Stockyard.", "stockyard.yml", `providers:\n  cohere:\n    type: cohere\n    api_key: \${COHERE_API_KEY}\n    # Requires AnthroFit-style adapter for non-OpenAI format`],
  ["deepseek", "DeepSeek", "Route DeepSeek through Stockyard.", "stockyard.yml", `providers:\n  deepseek:\n    base_url: https://api.deepseek.com/v1\n    api_key: \${DEEPSEEK_API_KEY}\n    # OpenAI-compatible`],
  ["xai", "xAI / Grok", "Route xAI Grok through Stockyard.", "stockyard.yml", `providers:\n  xai:\n    base_url: https://api.x.ai/v1\n    api_key: \${XAI_API_KEY}\n    # OpenAI-compatible`],
];

for (const [dir, name, desc, file, content] of providers) {
  gen(dir, name, "LLM Providers", "Provider config", desc, () => {
    write(dir, file, content);
    readme(dir, name, "LLM Providers", "Provider config",
      `${desc} Add ${name} as a provider in your Stockyard config for failover routing, cost tracking, and caching.`,
      `1. Add provider config to \`stockyard.yml\`\n2. Set API key in environment\n3. ${name} available as a routing target`,
      [file]);
  });
}

// ═══════════════════════════════════════════════════════════════
// Update master README
// ═══════════════════════════════════════════════════════════════

// Count everything
const allDirs = fs.readdirSync(DIR).filter(f => fs.statSync(path.join(DIR, f)).isDirectory());
const totalFiles = allDirs.reduce((sum, d) => {
  return sum + fs.readdirSync(path.join(DIR, d)).filter(f => !fs.statSync(path.join(DIR, d, f)).isDirectory()).length;
}, 0);

console.log(`\n✅ Created ${created} new integrations (${skipped} already existed)`);
console.log(`   Total integration directories: ${allDirs.length}`);
console.log(`   Total integration files: ${totalFiles}`);
