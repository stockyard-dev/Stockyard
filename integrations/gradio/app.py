import gradio as gr
from openai import OpenAI

client = OpenAI(base_url="http://localhost:4000/v1", api_key="any")

def chat(message, history):
    messages = [{"role": "user" if i%2==0 else "assistant", "content": m}
                for i, m in enumerate([m for pair in history for m in pair] + [message])]
    r = client.chat.completions.create(model="gpt-4o", messages=messages)
    return r.choices[0].message.content

demo = gr.ChatInterface(chat, title="Chat via Stockyard")
demo.launch()
