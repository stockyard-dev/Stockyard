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
        messages: [{ role: "user", content: `Summarize: ${event.data.text}` }],
      });
      return r.choices[0].message.content;
    });
    return { summary: result };
  }
);
