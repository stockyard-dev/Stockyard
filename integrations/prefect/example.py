from prefect import flow, task
from openai import OpenAI

client = OpenAI(base_url="http://localhost:4000/v1", api_key="any")

@task
def summarize(text: str) -> str:
    r = client.chat.completions.create(
        model="gpt-4o",
        messages=[{"role": "user", "content": f"Summarize: {text}"}],
    )
    return r.choices[0].message.content

@flow
def process_docs(docs: list[str]):
    return [summarize(d) for d in docs]
