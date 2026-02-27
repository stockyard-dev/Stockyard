/**
 * Stockyard + OpenAI Node.js SDK
 *
 * Just change the baseURL — everything else works exactly the same.
 */
import OpenAI from "openai";

const client = new OpenAI({
  baseURL: "http://localhost:4200/v1",
  // apiKey is still your OpenAI key
});

async function main() {
  // Standard OpenAI SDK usage
  const response = await client.chat.completions.create({
    model: "gpt-4o-mini",
    messages: [
      { role: "system", content: "You are a helpful assistant." },
      { role: "user", content: "What is the capital of France?" },
    ],
  });

  console.log(response.choices[0].message.content);

  // Streaming works too
  const stream = await client.chat.completions.create({
    model: "gpt-4o-mini",
    messages: [{ role: "user", content: "Count to 10" }],
    stream: true,
  });

  for await (const chunk of stream) {
    process.stdout.write(chunk.choices[0]?.delta?.content || "");
  }
  console.log();

  // Check traces
  const traces = await fetch("http://localhost:4200/api/observe/traces?limit=5");
  const data = await traces.json();
  for (const t of data.traces || []) {
    console.log(`  ${t.model} via ${t.provider} — ${t.latency_ms}ms`);
  }
}

main().catch(console.error);
