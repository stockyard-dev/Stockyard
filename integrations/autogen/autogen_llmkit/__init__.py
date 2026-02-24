"""
autogen-llmkit: AutoGen integration for LLMKit proxy.

Route all AutoGen agent LLM calls through LLMKit.

Usage:
    pip install autogen-llmkit

    from autogen_llmkit import llmkit_config

    config_list = llmkit_config(model="gpt-4o-mini")
    assistant = autogen.AssistantAgent("assistant", llm_config={"config_list": config_list})
"""

from __future__ import annotations

import os
from typing import Optional, List, Dict, Any


def llmkit_config(
    model: str = "gpt-4o-mini",
    proxy_url: str = "",
    api_key: str = "",
    temperature: float = 0.7,
    max_tokens: int = 2000,
) -> List[Dict[str, Any]]:
    """Generate an AutoGen-compatible config_list that routes through LLMKit.
    
    Args:
        model: Model name (LLMKit's ModelSwitch may override this).
        proxy_url: LLMKit proxy URL (default: env LLMKIT_PROXY_URL or localhost:4000).
        api_key: API key for proxy auth (default: env LLMKIT_API_KEY or "llmkit").
        temperature: Default temperature.
        max_tokens: Default max tokens.
    
    Returns:
        config_list suitable for AutoGen's llm_config parameter.
    
    Example:
        import autogen
        from autogen_llmkit import llmkit_config
        
        config_list = llmkit_config(model="gpt-4o-mini")
        
        assistant = autogen.AssistantAgent(
            "assistant",
            llm_config={"config_list": config_list, "temperature": 0.7},
        )
        user = autogen.UserProxyAgent("user", human_input_mode="NEVER")
        user.initiate_chat(assistant, message="Hello!")
    """
    base = proxy_url or os.getenv("LLMKIT_PROXY_URL", "http://localhost:4000")
    key = api_key or os.getenv("LLMKIT_API_KEY", "llmkit")

    return [
        {
            "model": model,
            "base_url": f"{base}/v1",
            "api_key": key,
            "temperature": temperature,
            "max_tokens": max_tokens,
        }
    ]


def llmkit_config_multi(
    models: List[str],
    proxy_url: str = "",
    api_key: str = "",
) -> List[Dict[str, Any]]:
    """Generate config_list with multiple models for AutoGen's failover.
    
    LLMKit handles failover internally, but AutoGen can also rotate models
    on its own. This gives you both layers of resilience.
    """
    base = proxy_url or os.getenv("LLMKIT_PROXY_URL", "http://localhost:4000")
    key = api_key or os.getenv("LLMKIT_API_KEY", "llmkit")

    return [
        {"model": m, "base_url": f"{base}/v1", "api_key": key}
        for m in models
    ]


def check_proxy(proxy_url: str = "") -> dict:
    """Check LLMKit proxy health and return status."""
    import requests
    base = proxy_url or os.getenv("LLMKIT_PROXY_URL", "http://localhost:4000")
    try:
        r = requests.get(f"{base}/health", timeout=3)
        return {"status": "ok" if r.status_code == 200 else "error", "url": base}
    except Exception as e:
        return {"status": "unreachable", "url": base, "error": str(e)}
