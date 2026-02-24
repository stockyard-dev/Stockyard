# Make.com + Stockyard

1. Add an HTTP module to your scenario
2. URL: http://your-stockyard-host:4000/v1/chat/completions
3. Method: POST
4. Headers: Content-Type: application/json
5. Body: {"model":"gpt-4o","messages":[{"role":"user","content":"{{input}}"}]}
6. Parse response: choices[0].message.content

