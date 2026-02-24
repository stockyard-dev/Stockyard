from pydantic_ai import Agent
from pydantic_ai.models.openai import OpenAIModel

model = OpenAIModel(
    "gpt-4o",
    base_url="http://localhost:4000/v1",
    api_key="any-string",
)

agent = Agent(model, system_prompt="You are helpful.")
