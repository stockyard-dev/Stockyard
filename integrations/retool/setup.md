# Retool + Stockyard

1. Resources > Add Resource > REST API
2. Base URL: http://your-stockyard-host:4000/v1
3. Create query: POST /chat/completions
4. Body: {"model":"gpt-4o","messages":[{"role":"user","content":{{textInput.value}}}]}

