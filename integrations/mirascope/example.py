from mirascope.core import openai

# Point at Stockyard
import os
os.environ["OPENAI_API_BASE"] = "http://localhost:4000/v1"

@openai.call("gpt-4o")
def recommend_book(genre: str) -> str:
    return f"Recommend a {genre} book"
