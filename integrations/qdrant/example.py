from qdrant_client import QdrantClient
from openai import OpenAI

embed_client = OpenAI(base_url="http://localhost:4000/v1", api_key="any")

def embed(texts):
    r = embed_client.embeddings.create(model="text-embedding-3-small", input=texts)
    return [e.embedding for e in r.data]

# Use embed() function when upserting to Qdrant
