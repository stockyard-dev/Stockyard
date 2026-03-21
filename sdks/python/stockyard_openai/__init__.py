"""Stockyard OpenAI SDK wrapper — drop-in replacement for OpenAI that routes through Stockyard."""

import os
try:
    from openai import OpenAI as _OpenAI, AsyncOpenAI as _AsyncOpenAI
except ImportError:
    raise ImportError("Install openai first: pip install openai")

__version__ = "1.0.0"

_DEFAULT_BASE = os.getenv("STOCKYARD_URL", "http://localhost:8080") + "/v1"


class OpenAI(_OpenAI):
    """OpenAI client that routes through Stockyard proxy."""

    def __init__(self, **kwargs):
        kwargs.setdefault("base_url", _DEFAULT_BASE)
        kwargs.setdefault("default_headers", {})
        kwargs["default_headers"]["X-Stockyard-Source"] = "python-sdk"
        super().__init__(**kwargs)


class AsyncOpenAI(_AsyncOpenAI):
    """Async OpenAI client that routes through Stockyard proxy."""

    def __init__(self, **kwargs):
        kwargs.setdefault("base_url", _DEFAULT_BASE)
        kwargs.setdefault("default_headers", {})
        kwargs["default_headers"]["X-Stockyard-Source"] = "python-sdk"
        super().__init__(**kwargs)
