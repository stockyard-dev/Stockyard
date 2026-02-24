"""
crewai-llmkit: CrewAI integration for LLMKit proxy.

Route all CrewAI agent LLM calls through LLMKit for cost caps, caching,
rate limiting, PII redaction, smart routing, and analytics.

Usage:
    pip install crewai-llmkit

    from crewai_llmkit import LLMKitLLM
    from crewai import Agent, Task, Crew

    llm = LLMKitLLM(model="gpt-4o-mini")
    agent = Agent(role="Researcher", llm=llm, ...)
"""

from __future__ import annotations

import os
import json
import requests
from typing import Any, Dict, List, Optional, Union

from langchain_core.callbacks import CallbackManagerForLLMRun
from langchain_core.language_models.chat_models import BaseChatModel
from langchain_core.messages import AIMessage, BaseMessage, HumanMessage, SystemMessage
from langchain_core.outputs import ChatGeneration, ChatResult
from pydantic import Field


def _msg_to_dict(msg: BaseMessage) -> dict:
    role_map = {HumanMessage: "user", AIMessage: "assistant", SystemMessage: "system"}
    role = role_map.get(type(msg), "user")
    return {"role": role, "content": str(msg.content)}


class LLMKitLLM(BaseChatModel):
    """LLMKit-proxied LLM for CrewAI agents.
    
    Drop-in replacement for any CrewAI LLM. Routes all calls through LLMKit
    proxy, giving every agent automatic cost caps, caching, rate limiting,
    PII redaction, smart routing, failover, and analytics.

    CrewAI agents are the highest token consumers — multi-agent workflows
    can burn through hundreds of dollars in API calls. LLMKit's caching
    alone typically saves 30-50% on repeated tool calls and reasoning steps.

    Example:
        from crewai import Agent, Task, Crew
        from crewai_llmkit import LLMKitLLM

        llm = LLMKitLLM(model="gpt-4o-mini", proxy_url="http://localhost:4000")
        
        researcher = Agent(
            role="Researcher",
            goal="Find accurate information",
            llm=llm,
        )
    """

    proxy_url: str = Field(
        default_factory=lambda: os.getenv("LLMKIT_PROXY_URL", "http://localhost:4000")
    )
    model: str = Field(default="gpt-4o-mini")
    temperature: float = Field(default=0.7)
    max_tokens: int = Field(default=2000)
    api_key: str = Field(
        default_factory=lambda: os.getenv("LLMKIT_API_KEY", "llmkit")
    )
    user_id: str = Field(default="crewai-agent")
    feature: str = Field(default="crewai")
    timeout: int = Field(default=180)

    @property
    def _llm_type(self) -> str:
        return "llmkit-crewai"

    def _generate(
        self,
        messages: List[BaseMessage],
        stop: Optional[List[str]] = None,
        run_manager: Optional[CallbackManagerForLLMRun] = None,
        **kwargs: Any,
    ) -> ChatResult:
        api_messages = [_msg_to_dict(m) for m in messages]
        
        headers = {
            "Content-Type": "application/json",
            "Authorization": f"Bearer {self.api_key}",
            "X-LLMKit-User": self.user_id,
            "X-LLMKit-Feature": self.feature,
        }

        body: Dict[str, Any] = {
            "model": self.model,
            "messages": api_messages,
            "temperature": self.temperature,
            "max_tokens": self.max_tokens,
            "stream": False,
        }
        if stop:
            body["stop"] = stop

        r = requests.post(
            f"{self.proxy_url}/v1/chat/completions",
            headers=headers,
            json=body,
            timeout=self.timeout,
        )
        r.raise_for_status()
        data = r.json()

        content = data["choices"][0]["message"]["content"]
        message = AIMessage(content=content)

        return ChatResult(
            generations=[ChatGeneration(
                message=message,
                generation_info={"usage": data.get("usage", {}), "model": data.get("model")},
            )],
            llm_output={"token_usage": data.get("usage", {}), "model_name": data.get("model")},
        )
