# Windmill Python script with Stockyard
import openai

def main(prompt: str):
    client = openai.OpenAI(
        base_url="http://stockyard:4000/v1",
        api_key="any-string",
    )
    r = client.chat.completions.create(
        model="gpt-4o",
        messages=[{"role": "user", "content": prompt}],
    )
    return r.choices[0].message.content
