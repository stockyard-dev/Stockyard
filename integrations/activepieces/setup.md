# Activepieces + Stockyard

Use the HTTP piece to call Stockyard:
1. Method: POST
2. URL: http://localhost:4000/v1/chat/completions
3. Headers: {"Content-Type": "application/json"}
4. Body: {"model":"gpt-4o","messages":[{"role":"user","content":"{{trigger.body}}"}]}

