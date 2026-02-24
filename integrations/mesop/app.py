import mesop as me
from openai import OpenAI

client = OpenAI(base_url="http://localhost:4000/v1", api_key="any")

@me.page(path="/")
def page():
    me.text("Chat via Stockyard")
    # Build Mesop chat UI with Stockyard-routed LLM calls
