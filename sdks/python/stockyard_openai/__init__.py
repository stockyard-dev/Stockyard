"""Drop-in OpenAI SDK wrapper that routes all traffic through Stockyard.

Usage:
    # Before:
    from openai import OpenAI
    # After:
    from stockyard_openai import OpenAI

    client = OpenAI(api_key="sk-...")
    # All requests now go through Stockyard automatically
"""

import os

from openai import OpenAI as _OriginalOpenAI
from openai import AsyncOpenAI as _OriginalAsyncOpenAI

_STOCKYARD_URL = os.environ.get("STOCKYARD_URL", "http://localhost:4200")
_ENABLED = os.environ.get("STOCKYARD_ENABLED", "true").lower() != "false"


class OpenAI(_OriginalOpenAI):
    """OpenAI client that routes through Stockyard proxy."""

    def __init__(self, **kwargs):
        if _ENABLED and "base_url" not in kwargs:
            kwargs["base_url"] = _STOCKYARD_URL + "/v1"
            if "default_headers" not in kwargs:
                kwargs["default_headers"] = {}
            kwargs["default_headers"]["X-Stockyard-Source"] = "sdk-python"
        super().__init__(**kwargs)


class AsyncOpenAI(_OriginalAsyncOpenAI):
    """Async OpenAI client that routes through Stockyard proxy."""

    def __init__(self, **kwargs):
        if _ENABLED and "base_url" not in kwargs:
            kwargs["base_url"] = _STOCKYARD_URL + "/v1"
            if "default_headers" not in kwargs:
                kwargs["default_headers"] = {}
            kwargs["default_headers"]["X-Stockyard-Source"] = "sdk-python-async"
        super().__init__(**kwargs)


# Re-export everything from openai
from openai import *  # noqa: F401,F403,E402
