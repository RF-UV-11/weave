"""OpenAI-chat-completions-compatible LLM client — the cloud/self-hosted
sibling `ollama_client.py`'s module docstring names ("a cloud provider
would be a sibling module behind the same chat()/chat_stream() shape").

Speaks the OpenAI `/v1/chat/completions` wire format directly over
`httpx` (already a dependency — no `openai` SDK needed for one endpoint
shape). Deliberately not OpenAI-specific despite the module name: any
backend exposing that same request/response shape works by pointing
OPENAI_BASE_URL at it — Azure OpenAI, Groq, OpenRouter, a self-hosted
vLLM/LM Studio/text-generation-webui server, even Ollama's own OpenAI-
compatible endpoint. Provider selection here is "which BASE_URL is
orchestrator configured with," not a one-module-per-vendor list that
grows forever — see `llm/router.py` for how a bot profile's
`llm_provider` picks between this module and `ollama_client`.

Credentials (OPENAI_API_KEY) are orchestrator's own configuration, the
same trust boundary as OLLAMA_HOST — a tenant's bot profile selects which
already-configured backend to use, it doesn't supply its own API key
through the platform (that would need per-tenant credential-vault
integration; tracked as real future work in
docs/architecture/ARCHITECTURE.md §3, not implemented here).
"""

import json
import os
from collections.abc import AsyncIterator
from typing import Any

import httpx

from .base import ChatResult, ToolCall

BASE_URL = os.environ.get("OPENAI_BASE_URL", "https://api.openai.com/v1").rstrip("/")
API_KEY = os.environ.get("OPENAI_API_KEY", "")
MODEL = os.environ.get("OPENAI_MODEL", "gpt-4o-mini")
_TIMEOUT_SECONDS = 30.0


def _headers() -> dict[str, str]:
    headers = {"Content-Type": "application/json"}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    return headers


def _to_openai_tool(name: str, description: str, input_schema: dict[str, Any]) -> dict[str, Any]:
    """Same function-calling shape ollama_client._to_ollama_tool builds —
    Ollama's own tool schema is already OpenAI-compatible, so this is
    intentionally near-identical, not a coincidence."""
    return {
        "type": "function",
        "function": {
            "name": name,
            "description": description,
            "parameters": input_schema or {"type": "object", "properties": {}},
        },
    }


def _parse_tool_calls(message: dict[str, Any]) -> list[ToolCall]:
    calls = []
    for tc in message.get("tool_calls") or []:
        fn = tc["function"]
        try:
            arguments = json.loads(fn.get("arguments") or "{}")
        except json.JSONDecodeError:
            # A model that emits malformed JSON arguments shouldn't crash
            # the turn — surfaced as "no arguments" (the tool call below
            # will then fail its own schema/lookup, a normal, already-
            # handled path in server/graph.py's _tool_node) rather than an
            # unhandled exception here.
            arguments = {}
        calls.append(ToolCall(name=fn["name"], arguments=arguments))
    return calls


async def chat(
    messages: list[dict[str, str]],
    *,
    tools: list[tuple[str, str, dict[str, Any]]] | None = None,
    model: str | None = None,
    transport: httpx.AsyncBaseTransport | None = None,
) -> ChatResult:
    """Same contract as ollama_client.chat(): one non-streaming turn,
    optionally offering tools, model overriding MODEL for this call.
    transport is test-only (httpx.MockTransport), same pattern as
    server/web_search.py's run_web_search — never passed in production."""
    payload: dict[str, Any] = {"model": model or MODEL, "messages": messages}
    if tools:
        payload["tools"] = [_to_openai_tool(n, d, s) for n, d, s in tools]

    async with httpx.AsyncClient(timeout=_TIMEOUT_SECONDS, transport=transport) as client:
        resp = await client.post(f"{BASE_URL}/chat/completions", json=payload, headers=_headers())
        resp.raise_for_status()
    message = resp.json()["choices"][0]["message"]
    return ChatResult(content=message.get("content") or "", tool_calls=_parse_tool_calls(message))


async def chat_stream(
    messages: list[dict[str, str]], *, model: str | None = None, transport: httpx.AsyncBaseTransport | None = None
) -> AsyncIterator[str]:
    """Same contract as ollama_client.chat_stream(): streams the final
    answer token-by-token, no tools offered (synthesis-only step).
    transport is test-only, same as chat() above."""
    payload = {"model": model or MODEL, "messages": messages, "stream": True}
    async with httpx.AsyncClient(timeout=_TIMEOUT_SECONDS, transport=transport) as client:
        async with client.stream("POST", f"{BASE_URL}/chat/completions", json=payload, headers=_headers()) as resp:
            resp.raise_for_status()
            async for line in resp.aiter_lines():
                if not line.startswith("data:"):
                    continue
                data = line[len("data:") :].strip()
                if not data or data == "[DONE]":
                    continue
                delta = json.loads(data)["choices"][0].get("delta") or {}
                if delta.get("content"):
                    yield delta["content"]
