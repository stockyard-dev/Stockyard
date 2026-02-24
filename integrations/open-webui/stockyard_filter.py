"""
title: LLMKit Cost Filter
author: LLMKit
version: 0.1.0
license: MIT
description: Adds LLM cost tracking and cache status to every Open WebUI response. Lightweight filter — no request rerouting.
requirements: requests
"""

import os
import requests
from typing import Optional
from pydantic import BaseModel, Field


class Filter:
    """
    LLMKit Cost Filter for Open WebUI.
    
    Lightweight filter that queries LLMKit after each response to show
    spend tracking, cache hit status, and budget alerts inline.
    
    Unlike the full pipeline, this doesn't reroute requests — it just
    adds observability. Use this alongside the pipeline or standalone.
    """

    class Valves(BaseModel):
        LLMKIT_PROXY_URL: str = Field(
            default="http://localhost:4000",
            description="LLMKit proxy base URL",
        )
        SHOW_SPEND: bool = Field(default=True, description="Show running spend total")
        SHOW_CACHE: bool = Field(default=True, description="Show cache hit/miss")
        BUDGET_WARNING_THRESHOLD: float = Field(
            default=0.8,
            description="Show warning when spend exceeds this fraction of daily budget (0-1)",
        )

    def __init__(self):
        self.name = "LLMKit Cost Filter"
        self.valves = self.Valves(
            LLMKIT_PROXY_URL=os.getenv("LLMKIT_PROXY_URL", "http://localhost:4000"),
        )

    def _query_spend(self) -> Optional[dict]:
        try:
            r = requests.get(
                f"{self.valves.LLMKIT_PROXY_URL}/api/spend",
                params={"project": "default"},
                timeout=2,
            )
            if r.status_code == 200:
                return r.json()
        except Exception:
            pass
        return None

    def _query_cache(self) -> Optional[dict]:
        try:
            r = requests.get(
                f"{self.valves.LLMKIT_PROXY_URL}/api/cache/stats",
                timeout=2,
            )
            if r.status_code == 200:
                return r.json()
        except Exception:
            pass
        return None

    def outlet(self, body: dict, __user__: Optional[dict] = None) -> dict:
        """Post-process: append cost info to the last assistant message."""
        messages = body.get("messages", [])
        if not messages:
            return body

        # Find last assistant message
        last_msg = None
        for msg in reversed(messages):
            if msg.get("role") == "assistant":
                last_msg = msg
                break

        if not last_msg:
            return body

        annotations = []

        if self.valves.SHOW_SPEND:
            spend = self._query_spend()
            if spend:
                today = spend.get("today", 0)
                cap = spend.get("daily_cap", 0)
                annotations.append(f"💰 Today: ${today:.4f}")
                if cap > 0:
                    pct = today / cap
                    if pct >= self.valves.BUDGET_WARNING_THRESHOLD:
                        annotations.append(f"⚠️ {pct*100:.0f}% of ${cap:.2f} daily budget")

        if self.valves.SHOW_CACHE:
            cache = self._query_cache()
            if cache:
                hits = cache.get("hits", 0)
                total = cache.get("total", 1)
                rate = (hits / total * 100) if total > 0 else 0
                saved = cache.get("estimated_savings", 0)
                annotations.append(f"📦 Cache: {rate:.0f}% hit (saved ${saved:.2f})")

        if annotations:
            footer = "\n\n---\n_" + " | ".join(annotations) + "_"
            last_msg["content"] = last_msg.get("content", "") + footer

        return body
