"""Ollama LLM client — local-first per OVERVIEW.md's tech stack table
("LLM access: OpenAI-compatible API + Ollama... same interface for cloud
and local models"). This module is the Ollama half; a cloud provider
would be a sibling module behind the same chat()/chat_stream() shape.
"""

import os
from collections.abc import AsyncIterator
from dataclasses import dataclass
from typing import Any

import ollama

MODEL = os.environ.get("OLLAMA_MODEL", "llama3.2:3b")
HOST = os.environ.get("OLLAMA_HOST", "http://127.0.0.1:11434")
# A dedicated embedding model, not the chat model above — nomic-embed-text
# is a small model purpose-built for embeddings (768 dimensions, matching
# core/configs's EMBEDDING_DIM default; the two must stay in sync, see
# server/semantic_memory.py). Never used for chat/tool-calling.
EMBED_MODEL = os.environ.get("OLLAMA_EMBED_MODEL", "nomic-embed-text")

_client = ollama.AsyncClient(host=HOST)


@dataclass
class ToolCall:
    name: str
    arguments: dict[str, Any]


@dataclass
class ChatResult:
    content: str
    tool_calls: list[ToolCall]


def _to_ollama_tool(name: str, description: str, input_schema: dict[str, Any]) -> dict[str, Any]:
    """Converts an MCP tool (name/description/JSON-schema) into the
    OpenAI-style function-calling shape Ollama expects."""
    return {
        "type": "function",
        "function": {
            "name": name,
            "description": description,
            "parameters": input_schema or {"type": "object", "properties": {}},
        },
    }


async def chat(
    messages: list[dict[str, str]],
    *,
    tools: list[tuple[str, str, dict[str, Any]]] | None = None,
) -> ChatResult:
    """One non-streaming turn, optionally offering tools for the model to
    call. tools is a list of (name, description, input_schema) — the
    tool's description travels with it into the model's context here,
    same as it must travel with the tool's *result* afterward (PLAN.md's
    tool-description requirement)."""
    ollama_tools = [_to_ollama_tool(n, d, s) for n, d, s in (tools or [])]
    resp = await _client.chat(model=MODEL, messages=messages, tools=ollama_tools or None)
    calls = [ToolCall(name=tc.function.name, arguments=dict(tc.function.arguments or {})) for tc in (resp.message.tool_calls or [])]
    return ChatResult(content=resp.message.content or "", tool_calls=calls)


async def chat_stream(messages: list[dict[str, str]]) -> AsyncIterator[str]:
    """Streams the final answer token-by-token — no tools offered here,
    this is only ever the synthesis step after any tool call is resolved."""
    async for chunk in await _client.chat(model=MODEL, messages=messages, stream=True):
        if chunk.message.content:
            yield chunk.message.content


async def embed(text: str) -> list[float]:
    """Embeds text for long-term/semantic memory (docs/architecture/
    ARCHITECTURE.md §5) — orchestrator computes the vector (it holds
    LLM/embedding-model access) and hands it to core's MemoryService;
    core never runs inference itself."""
    resp = await _client.embed(model=EMBED_MODEL, input=text)
    return list(resp.embeddings[0])
