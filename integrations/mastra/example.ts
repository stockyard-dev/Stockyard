import { Agent } from "@mastra/core";
import OpenAI from "openai";

const client = new OpenAI({
  baseURL: "http://localhost:4000/v1",
  apiKey: "any-string",
});

// Use client with Mastra agents
