import semantic_kernel as sk
from semantic_kernel.connectors.ai.open_ai import OpenAIChatCompletion

kernel = sk.Kernel()
kernel.add_service(OpenAIChatCompletion(
    ai_model_id="gpt-4o",
    async_client=None,
    api_key="any-string",
    org_id=None,
    default_headers=None,
    # Set base URL via environment: OPENAI_BASE_URL=http://localhost:4000/v1
))
