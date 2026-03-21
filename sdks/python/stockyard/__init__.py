"""Stockyard Python SDK — client for the Stockyard LLM proxy API."""

import json
import os
from urllib.request import Request, urlopen
from urllib.error import HTTPError
from typing import Any, Dict, List, Optional


class StockyardClient:
    """Client for the Stockyard API."""

    def __init__(self, base_url: str = None, api_key: str = None):
        self.base_url = (base_url or os.getenv("STOCKYARD_URL", "http://localhost:4200")).rstrip("/")
        self.api_key = api_key or os.getenv("STOCKYARD_ADMIN_KEY", "")

    def _request(self, method: str, path: str, body: Any = None) -> Any:
        url = f"{self.base_url}{path}"
        data = json.dumps(body).encode() if body else None
        req = Request(url, data=data, method=method)
        req.add_header("Content-Type", "application/json")
        if self.api_key:
            req.add_header("Authorization", f"Bearer {self.api_key}")
            req.add_header("X-Admin-Key", self.api_key)
        try:
            with urlopen(req, timeout=30) as resp:
                return json.loads(resp.read())
        except HTTPError as e:
            return json.loads(e.read())

    def _get(self, path: str) -> Any:
        return self._request("GET", path)

    def _post(self, path: str, body: Any = None) -> Any:
        return self._request("POST", path, body)

    def _put(self, path: str, body: Any = None) -> Any:
        return self._request("PUT", path, body)

    def _delete(self, path: str) -> Any:
        return self._request("DELETE", path)

    # --- Status ---

    def status(self) -> Dict:
        """Get system status."""
        return self._get("/api/status")

    def apps(self) -> List[Dict]:
        """List running apps."""
        return self._get("/api/apps").get("apps", [])

    # --- Proxy ---

    def list_modules(self) -> List[Dict]:
        """List all proxy modules."""
        return self._get("/api/proxy/modules").get("modules", [])

    def toggle_module(self, name: str, enabled: bool) -> Dict:
        """Enable or disable a proxy module."""
        return self._put(f"/api/proxy/modules/{name}", {"enabled": enabled})

    def list_providers(self) -> List[Dict]:
        """List configured LLM providers."""
        return self._get("/api/proxy/providers").get("providers", [])

    def provider_health(self) -> List[Dict]:
        """Check health of all providers."""
        return self._get("/api/proxy/providers/health").get("providers", [])

    # --- Observe ---

    def list_traces(self, limit: int = 20, model: str = None) -> List[Dict]:
        """List recent traces."""
        path = f"/api/observe/traces?limit={limit}"
        if model:
            path += f"&model={model}"
        return self._get(path).get("traces", [])

    def get_trace(self, trace_id: str) -> Dict:
        """Get a single trace."""
        return self._get(f"/api/observe/traces/{trace_id}")

    def get_costs(self, period: str = "today") -> Dict:
        """Get cost breakdown."""
        return self._get(f"/api/observe/costs?period={period}")

    def get_drift(self) -> Dict:
        """Get model drift detection results."""
        return self._get("/api/observe/drift")

    # --- Billing ---

    def list_customers(self) -> List[Dict]:
        """List billing customers."""
        return self._get("/api/billing/customers")

    def create_customer(self, name: str, email: str, external_id: str = "") -> Dict:
        """Create a billing customer."""
        return self._post("/api/billing/customers", {
            "name": name, "email": email, "external_id": external_id, "account_id": "default"
        })

    def query_usage(self, customer_id: str = None) -> Dict:
        """Query usage summary."""
        path = "/api/billing/usage/summary"
        if customer_id:
            path += f"?customer_id={customer_id}"
        return self._get(path)

    def credit_balance(self, customer_id: str) -> Dict:
        """Get credit balance for a customer."""
        return self._get(f"/api/billing/credits/balance?customer_id={customer_id}")

    # --- Team ---

    def list_team_members(self) -> List[Dict]:
        """List team members."""
        return self._get("/api/team/members")

    def invite_member(self, email: str, name: str = "", role: str = "viewer") -> Dict:
        """Invite a team member."""
        return self._post("/api/team/members", {"email": email, "name": name, "role": role})

    # --- Config ---

    def export_config(self) -> Dict:
        """Export full configuration."""
        return self._get("/api/config/export")

    def import_config(self, config: Dict) -> Dict:
        """Import configuration."""
        return self._post("/api/config/import", config)

    # --- Routing ---

    def list_routing_rules(self) -> List[Dict]:
        """List smart routing rules."""
        return self._get("/api/proxy/routing/rules").get("rules", [])

    def create_routing_rule(self, name: str, condition: Dict, action: Dict, priority: int = 0) -> Dict:
        """Create a routing rule."""
        return self._post("/api/proxy/routing/rules", {
            "name": name, "condition": condition, "action": action, "priority": priority
        })

    # --- Firewall ---

    def scan_text(self, text: str) -> Dict:
        """Scan text with the firewall."""
        return self._post("/api/firewall/scan", {"text": text})

    def firewall_stats(self) -> Dict:
        """Get firewall detection stats."""
        return self._get("/api/firewall/stats")

    # --- Chat ---

    def chat(self, model: str, messages: List[Dict], **kwargs) -> Dict:
        """Send a chat completion through the proxy."""
        body = {"model": model, "messages": messages, **kwargs}
        return self._post("/v1/chat/completions", body)
