from airflow.models import BaseOperator
from openai import OpenAI

class StockyardOperator(BaseOperator):
    def __init__(self, prompt, model="gpt-4o", **kwargs):
        super().__init__(**kwargs)
        self.prompt = prompt
        self.model = model

    def execute(self, context):
        client = OpenAI(
            base_url="http://stockyard:4000/v1",
            api_key="any-string",
        )
        r = client.chat.completions.create(
            model=self.model,
            messages=[{"role": "user", "content": self.prompt}],
        )
        return r.choices[0].message.content
