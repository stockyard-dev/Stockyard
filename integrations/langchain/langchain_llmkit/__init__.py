"""
langchain-llmkit: LangChain integration for LLMKit proxy.

Route all LangChain LLM calls through LLMKit for cost caps, caching,
rate limiting, PII redaction, smart routing, and analytics.

Usage:
    pip install langchain-llmkit

    from langchain_llmkit import ChatLLMKit

    llm = ChatLLMKit(
        proxy_url="http://localhost:4000",
        model="gpt-4o-mini",
    )
    response = llm.invoke("Hello!")
"""

from __future__ import annotations

import os
import json
import requests
from typing import Any, Dict, Iterator, List, Mapping, Optional

from langchain_core.callbacks import CallbackManagerForLLMRun
from langchain_core.language_models.chat_models import BaseChatModel
from langchain_core.messages import (
    AIMessage,
    AIMessageChunk,
    BaseMessage,
    ChatMessage,
    HumanMessage,
    SystemMessage,
)
from langchain_core.outputs import ChatGeneration, ChatGenerationChunk, ChatResult
from pydantic import Field


def _message_to_dict(msg: BaseMessage) -> dict:
    """Convert a LangChain message to OpenAI API format."""
    if isinstance(msg, HumanMessage):
        return {"role": "user", "content": msg.content}
    elif isinstance(msg, AIMessage):
        return {"role": "assistant", "content": msg.content}
    elif isinstance(msg, SystemMessage):
        return {"role": "system", "content": msg.content}
    elif isinstance(msg, ChatMessage):
        return {"role": msg.role, "content": msg.content}
    return {"role": "user", "content": str(msg.content)}


class ChatLLMKit(BaseChatModel):
    """LLMKit-proxied chat model for LangChain.
    
    All requests route through LLMKit proxy, automatically getting:
    - Cost caps (CostCap)
    - Response caching (CacheLayer)
    - Rate limiting (RateShield)
    - PII redaction (PromptGuard)
    - Smart routing (ModelSwitch)
    - Failover (FallbackRouter)
    - Analytics (LLMTap)
    
    Example:
        from langchain_llmkit import ChatLLMKit
        
        llm = ChatLLMKit(model="gpt-4o-mini")
        response = llm.invoke("What is LLMKit?")
    """

    proxy_url: str = Field(
        default_factory=lambda: os.getenv("LLMKIT_PROXY_URL", "http://localhost:4000")
    )
    model: str = Field(default="gpt-4o-mini")
    temperature: float = Field(default=0.7)
    max_tokens: int = Field(default=1000)
    api_key: str = Field(
        default_factory=lambda: os.getenv("LLMKIT_API_KEY", "llmkit")
    )
    user_id: str = Field(default="", description="User ID for UsagePulse metering")
    feature: str = Field(default="langchain", description="Feature tag for tracking")
    timeout: int = Field(default=120)

    @property
    def _llm_type(self) -> str:
        return "llmkit"

    @property
    def _identifying_params(self) -> Mapping[str, Any]:
        return {
            "proxy_url": self.proxy_url,
            "model": self.model,
            "temperature": self.temperature,
            "max_tokens": self.max_tokens,
        }

    def _build_headers(self) -> dict:
        headers = {
            "Content-Type": "application/json",
            "Authorization": f"Bearer {self.api_key}",
        }
        if self.user_id:
            headers["X-LLMKit-User"] = self.user_id
        if self.feature:
            headers["X-LLMKit-Feature"] = self.feature
        return headers

    def _generate(
        self,
        messages: List[BaseMessage],
        stop: Optional[List[str]] = None,
        run_manager: Optional[CallbackManagerForLLMRun] = None,
        **kwargs: Any,
    ) -> ChatResult:
        """Generate a chat response via LLMKit proxy."""
        api_messages = [_message_to_dict(m) for m in messages]
        
        body: Dict[str, Any] = {
            "model": self.model,
            "messages": api_messages,
            "temperature": self.temperature,
            "max_tokens": self.max_tokens,
            "stream": False,
        }
        if stop:
            body["stop"] = stop
        body.update(kwargs)

        r = requests.post(
            f"{self.proxy_url}/v1/chat/completions",
            headers=self._build_headers(),
            json=body,
            timeout=self.timeout,
        )
        r.raise_for_status()
        data = r.json()

        content = data["choices"][0]["message"]["content"]
        usage = data.get("usage", {})

        message = AIMessage(content=content)
        generation = ChatGeneration(
            message=message,
            generation_info={
                "model": data.get("model", self.model),
                "usage": usage,
                "llmkit_id": data.get("id", ""),
            },
        )

        return ChatResult(
            generations=[generation],
            llm_output={
                "token_usage": usage,
                "model_name": data.get("model", self.model),
            },
        )

    def _stream(
        self,
        messages: List[BaseMessage],
        stop: Optional[List[str]] = None,
        run_manager: Optional[CallbackManagerForLLMRun] = None,
        **kwargs: Any,
    ) -> Iterator[ChatGenerationChunk]:
        """Stream a chat response via LLMKit proxy."""
        api_messages = [_message_to_dict(m) for m in messages]

        body: Dict[str, Any] = {
            "model": self.model,
            "messages": api_messages,
            "temperature": self.temperature,
            "max_tokens": self.max_tokens,
            "stream": True,
        }
        if stop:
            body["stop"] = stop
        body.update(kwargs)

        r = requests.post(
            f"{self.proxy_url}/v1/chat/completions",
            headers=self._build_headers(),
            json=body,
            stream=True,
            timeout=self.timeout,
        )
        r.raise_for_status()

        for line in r.iter_lines(decode_unicode=True):
            if not line or not line.startswith("data: "):
                continue
            data_str = line[6:]
            if data_str.strip() == "[DONE]":
                return
            try:
                chunk = json.loads(data_str)
                delta = chunk.get("choices", [{}])[0].get("delta", {})
                content = delta.get("content", "")
                if content:
                    message_chunk = AIMessageChunk(content=content)
                    yield ChatGenerationChunk(message=message_chunk)
                    if run_manager:
                        run_manager.on_llm_new_token(content)
            except json.JSONDecodeError:
                continue


# Convenience functions
def get_spend(proxy_url: str = "http://localhost:4000", project: str = "default") -> dict:
    """Get current LLMKit spend."""
    r = requests.get(f"{proxy_url}/api/spend", params={"project": project}, timeout=5)
    r.raise_for_status()
    return r.json()


def get_cache_stats(proxy_url: str = "http://localhost:4000") -> dict:
    """Get LLMKit cache statistics."""
    r = requests.get(f"{proxy_url}/api/cache/stats", timeout=5)
    r.raise_for_status()
    return r.json()


def check_health(proxy_url: str = "http://localhost:4000") -> bool:
    """Check if LLMKit proxy is running."""
    try:
        r = requests.get(f"{proxy_url}/health", timeout=3)
        return r.status_code == 200
    except Exception:
        return False
