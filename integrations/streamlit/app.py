import streamlit as st
from openai import OpenAI

client = OpenAI(base_url="http://localhost:4000/v1", api_key="any")

st.title("Chat via Stockyard")

if prompt := st.chat_input("Ask anything"):
    st.chat_message("user").write(prompt)
    r = client.chat.completions.create(
        model="gpt-4o",
        messages=[{"role": "user", "content": prompt}],
    )
    st.chat_message("assistant").write(r.choices[0].message.content)
