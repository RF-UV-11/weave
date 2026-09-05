"""Ollama LLM client — local-first per OVERVIEW.md's tech stack table
("LLM access: OpenAI-compatible API + Ollama... same interface for cloud
and local models") and orchestrator's default provider (`llm/router.py`)
when a tenant's bot profile hasn't chosen one. This module is the Ollama
half; `openai_compat_client.py` is the cloud/self-hosted sibling behind
the same chat()/chat_stream() shape — `router.py` is what lets callers
treat the two interchangeably.
"""

import os
from collections.abc import AsyncIterator
from typing import Any

import ollama

from .base import ChatResult, ToolCall

MODEL = os.environ.get("OLLAMA_MODEL", "llama3.2:3b")
HOST = os.environ.get("OLLAMA_HOST", "http://127.0.0.1:11434")
# A dedicated embedding model, not the chat model above — nomic-embed-text
# is a small model purpose-built for embeddings (768 dimensions, matching
# core/configs's EMBEDDING_DIM default; the two must stay in sync, see
# server/semantic_memory.py). Never used for chat/tool-calling, and never
# provider-selectable per bot profile the way chat/chat_stream are below
# — semantic memory embeds on Ollama regardless of a profile's
# llm_provider (docs/architecture/ARCHITECTURE.md §5's "core is the only
# tier with real infra credentials" boundary doesn't change here; making
# embeddings provider-selectable too is real future work, not done here).
EMBED_MODEL = os.environ.get("OLLAMA_EMBED_MODEL", "nomic-embed-text")

_client = ollama.AsyncClient(host=HOST)


def _prepare_messages(messages: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """Translates chat_service.py's internal `image_attachments` key
    (attachments/process.py's ImageAttachment list, provider-agnostic)
    into Ollama's own message shape — a per-message `images` list of raw
    base64 strings, no data-URI prefix (openai_compat_client.py's
    equivalent conversion looks different because OpenAI's wire format
    for image input differs). Messages without `image_attachments` pass
    through unchanged; this only copies the messages that need
    translating, so callers with no attachments pay nothing extra."""
    if not any("image_attachments" in m for m in messages):
        return messages
    prepared = []
    for m in messages:
        images = m.get("image_attachments")
        if not images:
            prepared.append(m)
            continue
        m = {k: v for k, v in m.items() if k != "image_attachments"}
        m["images"] = [img.data_b64 for img in images]
        prepared.append(m)
    return prepared


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
    model: str | None = None,
) -> ChatResult:
    """One non-streaming turn, optionally offering tools for the model to
    call. tools is a list of (name, description, input_schema) — the
    tool's description travels with it into the model's context here,
    same as it must travel with the tool's *result* afterward (PLAN.md's
    tool-description requirement). model overrides MODEL for this call —
    how a bot profile's own `llm_model` (BotProfile.llm_model) takes
    effect, falling back to the module default when unset (`""`/`None`)."""
    ollama_tools = [_to_ollama_tool(n, d, s) for n, d, s in (tools or [])]
    resp = await _client.chat(model=model or MODEL, messages=_prepare_messages(messages), tools=ollama_tools or None)
    calls = [ToolCall(name=tc.function.name, arguments=dict(tc.function.arguments or {})) for tc in (resp.message.tool_calls or [])]
    return ChatResult(content=resp.message.content or "", tool_calls=calls)


async def chat_stream(messages: list[dict[str, str]], *, model: str | None = None) -> AsyncIterator[str]:
    """Streams the final answer token-by-token — no tools offered here,
    this is only ever the synthesis step after any tool call is resolved.
    model overrides MODEL for this call, same convention as chat() above."""
    async for chunk in await _client.chat(model=model or MODEL, messages=_prepare_messages(messages), stream=True):
        if chunk.message.content:
            yield chunk.message.content


async def embed(text: str) -> list[float]:
    """Embeds text for long-term/semantic memory (docs/architecture/
    ARCHITECTURE.md §5) — orchestrator computes the vector (it holds
    LLM/embedding-model access) and hands it to core's MemoryService;
    core never runs inference itself."""
    resp = await _client.embed(model=EMBED_MODEL, input=text)
    return list(resp.embeddings[0])
