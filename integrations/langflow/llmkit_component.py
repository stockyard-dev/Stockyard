"""
LLMKit Component for Langflow.

Drop-in replacement for the ChatOpenAI component that routes through LLMKit proxy.
Drag this into any Langflow flow to get cost caps, caching, and analytics.

Install: Copy to Langflow custom components directory.
"""

from langflow.custom import Component
from langflow.io import MessageTextInput, Output, StrInput, FloatInput, IntInput
from langchain_openai import ChatOpenAI
from langchain_core.messages import HumanMessage


class LLMKitComponent(Component):
    display_name = "LLMKit Chat"
    description = "Chat completion via LLMKit proxy (cost caps, caching, rate limiting, analytics)"
    icon = "🔧"
    name = "LLMKitChat"

    inputs = [
        MessageTextInput(name="input_value", display_name="Input", required=True),
        StrInput(
            name="proxy_url",
            display_name="LLMKit Proxy URL",
            value="http://localhost:4000/v1",
            info="Base URL of your LLMKit proxy",
        ),
        StrInput(
            name="model",
            display_name="Model",
            value="gpt-4o-mini",
            info="Model name (LLMKit's ModelSwitch may override)",
        ),
        FloatInput(
            name="temperature",
            display_name="Temperature",
            value=0.7,
            range_spec={"min": 0, "max": 2, "step": 0.1},
        ),
        IntInput(
            name="max_tokens",
            display_name="Max Tokens",
            value=1000,
        ),
        StrInput(
            name="system_message",
            display_name="System Message",
            value="",
            info="Optional system message prepended to all requests",
        ),
    ]

    outputs = [
        Output(display_name="Response", name="response", method="generate"),
    ]

    def generate(self) -> str:
        llm = ChatOpenAI(
            base_url=self.proxy_url,
            api_key="llmkit",  # LLMKit handles auth
            model=self.model,
            temperature=self.temperature,
            max_tokens=self.max_tokens,
        )

        messages = []
        if self.system_message:
            from langchain_core.messages import SystemMessage
            messages.append(SystemMessage(content=self.system_message))
        messages.append(HumanMessage(content=self.input_value))

        response = llm.invoke(messages)
        self.status = response.content
        return response.content
