# pip install llama-index-llms-openai llama-index-embeddings-openai
from llama_index.llms.openai import OpenAI
from llama_index.embeddings.openai import OpenAIEmbedding

llm = OpenAI(
    model="gpt-4o",
    api_base="http://localhost:4000/v1",
    api_key="any-string",
)

embed = OpenAIEmbedding(
    model="text-embedding-3-small",
    api_base="http://localhost:4000/v1",
    api_key="any-string",
)
