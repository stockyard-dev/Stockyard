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
}
