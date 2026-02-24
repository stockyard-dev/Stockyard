"""
title: LLMKit Pipeline
author: LLMKit
version: 0.1.0
license: MIT
description: Route all Open WebUI LLM calls through LLMKit proxy for cost caps, caching, rate limiting, PII redaction, smart routing, and analytics.
requirements: requests
"""

import os
import json
import requests
from typing import Optional, List, Union, Generator, Iterator
from pydantic import BaseModel, Field


class Pipeline:
    """
    LLMKit Pipeline for Open WebUI.
    
    Intercepts all LLM requests and routes them through a local LLMKit proxy,
    giving you cost caps, caching, rate limiting, PII redaction, smart routing,
    failover, analytics, and more — all without changing your models or config.

    Setup:
      1. Run LLMKit: npx @llmkit/llmkit (or docker run ghcr.io/llmkit/llmkit)
      2. Install this pipeline in Open WebUI → Admin → Pipelines
      3. Configure the proxy URL below (default: http://localhost:4000)
    """

    class Valves(BaseModel):
        LLMKIT_PROXY_URL: str = Field(
            default="http://localhost:4000",
            description="LLMKit proxy base URL (e.g. http://localhost:4000)",
        )
        LLMKIT_API_KEY: str = Field(
            default="",
            description="Optional API key for LLMKit proxy auth",
        )
        PASS_THROUGH_MODELS: str = Field(
            default="",
            description="Comma-separated model names to bypass LLMKit (empty = proxy all)",
        )
        SHOW_COST_IN_RESPONSE: bool = Field(
            default=False,
            description="Append cost info to responses (from X-LLMKit-Cost header)",
        )

    def __init__(self):
        self.name = "LLMKit"
        self.valves = self.Valves(
            LLMKIT_PROXY_URL=os.getenv("LLMKIT_PROXY_URL", "http://localhost:4000"),
            LLMKIT_API_KEY=os.getenv("LLMKIT_API_KEY", ""),
        )

    async def on_startup(self):
        """Verify LLMKit proxy is reachable on startup."""
        try:
            r = requests.get(f"{self.valves.LLMKIT_PROXY_URL}/health", timeout=3)
            if r.status_code == 200:
                print(f"[LLMKit] ✓ Connected to proxy at {self.valves.LLMKIT_PROXY_URL}")
            else:
                print(f"[LLMKit] ⚠ Proxy returned {r.status_code} — requests will fail")
        except Exception as e:
            print(f"[LLMKit] ⚠ Cannot reach proxy at {self.valves.LLMKIT_PROXY_URL}: {e}")
            print("[LLMKit] Start LLMKit: npx @llmkit/llmkit")

    async def on_shutdown(self):
        pass

    def _should_bypass(self, model: str) -> bool:
        """Check if this model should bypass the proxy."""
        if not self.valves.PASS_THROUGH_MODELS:
            return False
        bypass_list = [m.strip() for m in self.valves.PASS_THROUGH_MODELS.split(",")]
        return model in bypass_list

    def _build_headers(self) -> dict:
        headers = {"Content-Type": "application/json"}
        if self.valves.LLMKIT_API_KEY:
            headers["Authorization"] = f"Bearer {self.valves.LLMKIT_API_KEY}"
        return headers

    def pipe(
        self,
        body: dict,
        __user__: Optional[dict] = None,
    ) -> Union[str, Generator, Iterator]:
        """
        Main pipeline handler. Routes requests through LLMKit proxy.
        Supports both streaming and non-streaming.
        """
        model = body.get("model", "")

        if self._should_bypass(model):
            # Return None to let Open WebUI handle normally
            return None

        proxy_url = f"{self.valves.LLMKIT_PROXY_URL}/v1/chat/completions"
        headers = self._build_headers()

        # Add user context for UsagePulse metering
        if __user__:
            headers["X-LLMKit-User"] = __user__.get("id", "anonymous")
            headers["X-LLMKit-Feature"] = "open-webui"

        streaming = body.get("stream", False)

        if streaming:
            return self._stream_response(proxy_url, headers, body)
        else:
            return self._sync_response(proxy_url, headers, body)

    def _sync_response(self, url: str, headers: dict, body: dict) -> str:
        """Non-streaming request through LLMKit proxy."""
        try:
            body["stream"] = False
            r = requests.post(url, headers=headers, json=body, timeout=120)
            r.raise_for_status()
            data = r.json()

            content = data["choices"][0]["message"]["content"]

            if self.valves.SHOW_COST_IN_RESPONSE:
                cost = r.headers.get("X-LLMKit-Cost", "")
                if cost:
                    content += f"\n\n---\n_Cost: ${cost}_"

            return content

        except requests.exceptions.ConnectionError:
            return (
                "⚠️ **LLMKit proxy not reachable.** "
                f"Start it with: `npx @llmkit/llmkit` "
                f"(expected at {self.valves.LLMKIT_PROXY_URL})"
            )
        except requests.exceptions.HTTPError as e:
            status = e.response.status_code if e.response else "unknown"
            detail = ""
            try:
                detail = e.response.json().get("error", {}).get("message", "")
            except Exception:
                pass
            return f"⚠️ **LLMKit error ({status}):** {detail or str(e)}"
        except Exception as e:
            return f"⚠️ **LLMKit pipeline error:** {str(e)}"

    def _stream_response(
        self, url: str, headers: dict, body: dict
    ) -> Generator[str, None, None]:
        """Streaming request through LLMKit proxy with SSE pass-through."""
        try:
            body["stream"] = True
            r = requests.post(url, headers=headers, json=body, stream=True, timeout=120)
            r.raise_for_status()

            for line in r.iter_lines(decode_unicode=True):
                if not line:
                    continue
                if line.startswith("data: "):
                    data_str = line[6:]
                    if data_str.strip() == "[DONE]":
                        return
                    try:
                        chunk = json.loads(data_str)
                        delta = chunk.get("choices", [{}])[0].get("delta", {})
                        content = delta.get("content", "")
                        if content:
                            yield content
                    except json.JSONDecodeError:
                        continue

        except requests.exceptions.ConnectionError:
            yield (
                "⚠️ **LLMKit proxy not reachable.** "
                f"Start it with: `npx @llmkit/llmkit`"
            )
        except Exception as e:
            yield f"⚠️ **LLMKit error:** {str(e)}"
