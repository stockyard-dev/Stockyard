# Bubble.io + Stockyard

1. Use the API Connector plugin in Bubble
2. Add new API:
   - Name: Stockyard
   - Base URL: http://your-stockyard-host:4000/v1
   - Authentication: None (Stockyard handles it)
3. Add call: POST /chat/completions
4. Body: {"model":"gpt-4o","messages":[{"role":"user","content":"<prompt>"}]}

