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

