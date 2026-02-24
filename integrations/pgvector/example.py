from openai import OpenAI
import psycopg2

embed_client = OpenAI(base_url="http://localhost:4000/v1", api_key="any")

def embed_and_store(text, conn):
    r = embed_client.embeddings.create(model="text-embedding-3-small", input=[text])
    vector = r.data[0].embedding
    cur = conn.cursor()
    cur.execute("INSERT INTO docs (content, embedding) VALUES (%s, %s)", (text, vector))
    conn.commit()
