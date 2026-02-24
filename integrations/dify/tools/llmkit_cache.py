"""LLMKit Cache Stats Tool for Dify workflows."""

import requests
from typing import Any
from core.tools.tool.builtin_tool import BuiltinTool
from core.tools.entities.tool_entities import ToolInvokeMessage


class LLMKitCacheTool(BuiltinTool):
    def _invoke(self, user_id: str, tool_parameters: dict[str, Any]) -> ToolInvokeMessage:
        base_url = self.runtime.credentials.get("base_url", "http://localhost:4000")
        try:
            r = requests.get(f"{base_url}/api/cache/stats", timeout=5)
            r.raise_for_status()
            data = r.json()
            hits = data.get("hits", 0)
            misses = data.get("misses", 0)
            total = hits + misses
            rate = (hits / total * 100) if total > 0 else 0
            saved = data.get("estimated_savings", 0)
            return self.create_text_message(
                f"Cache Stats: {rate:.1f}% hit rate ({hits} hits, {misses} misses)\n"
                f"Estimated savings: ${saved:.2f}\n"
                f"Entries: {data.get('entries', 0)}"
            )
        except Exception as e:
            return self.create_text_message(f"Error: {e}")
