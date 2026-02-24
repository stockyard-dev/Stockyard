import chromadb
from openai import OpenAI

client = OpenAI(base_url="http://localhost:4000/v1", api_key="any")

# Custom embedding function using Stockyard-cached embeddings
class StockyardEmbedding(chromadb.EmbeddingFunction):
    def __call__(self, input):
        r = client.embeddings.create(model="text-embedding-3-small", input=input)
        return [e.embedding for e in r.data]

collection = chromadb.Client().create_collection("docs", embedding_function=StockyardEmbedding())
