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
});
