"""LLMKit Spend Tracking Tool for Dify workflows."""

import requests
from typing import Any
from core.tools.tool.builtin_tool import BuiltinTool
from core.tools.entities.tool_entities import ToolInvokeMessage


class LLMKitSpendTool(BuiltinTool):
    def _invoke(self, user_id: str, tool_parameters: dict[str, Any]) -> ToolInvokeMessage:
        base_url = self.runtime.credentials.get("base_url", "http://localhost:4000")
        project = tool_parameters.get("project", "default")

        try:
            r = requests.get(f"{base_url}/api/spend", params={"project": project}, timeout=5)
            r.raise_for_status()
            data = r.json()
            today = data.get("today", 0)
            month = data.get("month", 0)
            daily_cap = data.get("daily_cap", 0)
            monthly_cap = data.get("monthly_cap", 0)

            text = f"LLMKit Spend Report\n"
            text += f"Today: ${today:.4f}"
            if daily_cap > 0:
                text += f" / ${daily_cap:.2f} ({today/daily_cap*100:.0f}%)"
            text += f"\nThis month: ${month:.4f}"
            if monthly_cap > 0:
                text += f" / ${monthly_cap:.2f} ({month/monthly_cap*100:.0f}%)"

            return self.create_text_message(text)
        except Exception as e:
            return self.create_text_message(f"Error querying LLMKit spend: {e}")
