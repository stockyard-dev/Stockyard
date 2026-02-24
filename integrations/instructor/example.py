import instructor
from openai import OpenAI

client = instructor.from_openai(
    OpenAI(
        base_url="http://localhost:4000/v1",
        api_key="any-string",
    )
)

# Stockyard's StructuredShield validates JSON automatically
user = client.chat.completions.create(
    model="gpt-4o",
    response_model=User,
    messages=[{"role": "user", "content": "Extract: John is 25"}],
)
