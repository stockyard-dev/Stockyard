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
});
