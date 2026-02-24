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
}
