from dagster import asset
from openai import OpenAI

client = OpenAI(base_url="http://localhost:4000/v1", api_key="any")

@asset
def summarized_docs(raw_docs):
    results = []
    for doc in raw_docs:
        r = client.chat.completions.create(
            model="gpt-4o",
            messages=[{"role": "user", "content": f"Summarize: {doc}"}],
        )
        results.append(r.choices[0].message.content)
    return results
