"""Shared result types every LLM provider module returns — kept separate
from any one provider so `server/graph.py`/`server/chat_service.py` can
import them without importing a specific provider (e.g. `ollama_client`,
which opens a client at module-import time), and so `isinstance`/equality
on a `ToolCall`/`ChatResult` means the same thing regardless of which
provider produced it (`router.py` picks the provider per turn — every
provider must hand back exactly this shape)."""

from dataclasses import dataclass
from typing import Any


@dataclass
class ToolCall:
    name: str
    arguments: dict[str, Any]


@dataclass
class ChatResult:
    content: str
    tool_calls: list[ToolCall]
