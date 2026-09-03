from types import SimpleNamespace
from unittest.mock import AsyncMock

import llm.ollama_client as ollama_client_module
from llm.ollama_client import _to_ollama_tool, embed


def test_to_ollama_tool_shape():
    tool = _to_ollama_tool("book_appointment", "Book a slot", {"type": "object", "properties": {}})
    assert tool == {
        "type": "function",
        "function": {
            "name": "book_appointment",
            "description": "Book a slot",
            "parameters": {"type": "object", "properties": {}},
        },
    }


def test_to_ollama_tool_defaults_empty_schema():
    tool = _to_ollama_tool("noop", "does nothing", {})
    assert tool["function"]["parameters"] == {"type": "object", "properties": {}}


async def test_embed_calls_embed_model_and_returns_vector(monkeypatch):
    fake_embed = AsyncMock(return_value=SimpleNamespace(embeddings=[[0.1, 0.2, 0.3]]))
    monkeypatch.setattr(ollama_client_module._client, "embed", fake_embed)

    vector = await embed("hello world")

    assert vector == [0.1, 0.2, 0.3]
    fake_embed.assert_awaited_once_with(model=ollama_client_module.EMBED_MODEL, input="hello world")
