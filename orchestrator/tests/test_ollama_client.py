from types import SimpleNamespace
from unittest.mock import AsyncMock

import llm.ollama_client as ollama_client_module
from attachments.process import ImageAttachment
from llm.ollama_client import _prepare_messages, _to_ollama_tool, embed


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


async def test_chat_uses_module_default_model_when_none_given(monkeypatch):
    fake_message = SimpleNamespace(content="hi", tool_calls=None)
    fake_chat = AsyncMock(return_value=SimpleNamespace(message=fake_message))
    monkeypatch.setattr(ollama_client_module._client, "chat", fake_chat)

    await ollama_client_module.chat([{"role": "user", "content": "hi"}])

    assert fake_chat.call_args.kwargs["model"] == ollama_client_module.MODEL


async def test_chat_model_override_takes_precedence(monkeypatch):
    fake_message = SimpleNamespace(content="hi", tool_calls=None)
    fake_chat = AsyncMock(return_value=SimpleNamespace(message=fake_message))
    monkeypatch.setattr(ollama_client_module._client, "chat", fake_chat)

    await ollama_client_module.chat([{"role": "user", "content": "hi"}], model="llama3.2:1b")

    assert fake_chat.call_args.kwargs["model"] == "llama3.2:1b"


async def test_chat_stream_model_override_takes_precedence(monkeypatch):
    async def fake_stream_gen():
        yield SimpleNamespace(message=SimpleNamespace(content="hi"))

    async def fake_chat(*, model, messages, stream):
        assert model == "llama3.2:1b"
        return fake_stream_gen()

    monkeypatch.setattr(ollama_client_module._client, "chat", fake_chat)

    chunks = [c async for c in ollama_client_module.chat_stream([{"role": "user", "content": "hi"}], model="llama3.2:1b")]

    assert chunks == ["hi"]


def test_prepare_messages_passes_through_without_image_attachments():
    messages = [{"role": "user", "content": "hi"}]
    assert _prepare_messages(messages) is messages


def test_prepare_messages_converts_image_attachments_to_ollama_images():
    messages = [
        {"role": "system", "content": "be helpful"},
        {
            "role": "user",
            "content": "what's in this photo?",
            "image_attachments": [ImageAttachment(mime_type="image/jpeg", data_b64="ZmFrZQ==")],
        },
    ]

    prepared = _prepare_messages(messages)

    assert prepared[0] == {"role": "system", "content": "be helpful"}
    assert prepared[1] == {
        "role": "user",
        "content": "what's in this photo?",
        "images": ["ZmFrZQ=="],
    }
    assert "image_attachments" not in prepared[1]


async def test_chat_forwards_ollama_shaped_images(monkeypatch):
    fake_message = SimpleNamespace(content="a cat", tool_calls=None)
    fake_chat = AsyncMock(return_value=SimpleNamespace(message=fake_message))
    monkeypatch.setattr(ollama_client_module._client, "chat", fake_chat)

    await ollama_client_module.chat(
        [{"role": "user", "content": "what's this?", "image_attachments": [ImageAttachment("image/png", "ZmFrZQ==")]}]
    )

    sent_messages = fake_chat.call_args.kwargs["messages"]
    assert sent_messages == [{"role": "user", "content": "what's this?", "images": ["ZmFrZQ=="]}]
