from haystack.components.generators.chat import OpenAIChatGenerator
from haystack.utils import Secret

generator = OpenAIChatGenerator(
    api_key=Secret.from_token("any-string"),
    api_base_url="http://localhost:4000/v1",
    model="gpt-4o",
)
