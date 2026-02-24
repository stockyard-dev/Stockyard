"""
LLMKit Model Provider for Dify.

Registers LLMKit proxy as an OpenAI-compatible model provider.
All requests route through LLMKit, getting cost caps, caching, etc. automatically.
"""

import os
from typing import Optional, Generator
from core.model_runtime.entities.model_entities import ModelType
from core.model_runtime.model_providers.openai_api_compatible.openai_api_compatible import (
    OAIAPICompatModelProvider,
)


class LLMKitModelProvider(OAIAPICompatModelProvider):
    """LLMKit as an OpenAI-compatible model provider for Dify."""

    def validate_provider_credentials(self, credentials: dict) -> None:
        """Validate that LLMKit proxy is reachable."""
        import requests

        base_url = credentials.get("base_url", "http://localhost:4000")
        try:
            r = requests.get(f"{base_url}/health", timeout=5)
            if r.status_code != 200:
                raise ValueError(f"LLMKit proxy returned {r.status_code}")
        except requests.ConnectionError:
            raise ValueError(
                f"Cannot connect to LLMKit at {base_url}. "
                "Start it with: npx @llmkit/llmkit"
            )

    @staticmethod
    def get_provider_schema() -> dict:
        return {
            "provider": "llmkit",
            "label": {"en_US": "LLMKit Proxy", "zh_Hans": "LLMKit 代理"},
            "description": {
                "en_US": "Route through LLMKit for cost caps, caching, rate limiting, and analytics",
                "zh_Hans": "通过LLMKit代理路由，实现成本上限、缓存、速率限制和分析",
            },
            "icon_small": {"en_US": "icon_s.svg"},
            "icon_large": {"en_US": "icon_l.svg"},
            "supported_model_types": [ModelType.LLM],
            "credential_form_schemas": [
                {
                    "variable": "base_url",
                    "label": {"en_US": "LLMKit Proxy URL"},
                    "type": "text-input",
                    "required": True,
                    "default": "http://localhost:4000/v1",
                    "placeholder": "http://localhost:4000/v1",
                },
                {
                    "variable": "api_key",
                    "label": {"en_US": "API Key (any non-empty string)"},
                    "type": "secret-input",
                    "required": True,
                    "default": "llmkit",
                    "placeholder": "llmkit",
                },
            ],
            "model_credential_schema": {
                "model": {
                    "label": {"en_US": "Model Name"},
                    "placeholder": {"en_US": "gpt-4o-mini"},
                },
            },
        }
