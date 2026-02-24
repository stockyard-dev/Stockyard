import guidance

# Set OpenAI base URL to Stockyard
import os
os.environ["OPENAI_API_BASE"] = "http://localhost:4000/v1"
os.environ["OPENAI_API_KEY"] = "any-string"

model = guidance.models.OpenAI("gpt-4o")
