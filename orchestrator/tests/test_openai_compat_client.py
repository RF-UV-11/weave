import json

import httpx

from llm import openai_compat_client as client_module


def transport_for(handler):
    return httpx.MockTransport(handler)


async def test_chat_returns_content_and_no_tool_calls():
    def handler(request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content)
        assert body["model"] == client_module.MODEL
        assert body["messages"] == [{"role": "user", "content": "hi"}]
        assert "tools" not in body
        return httpx.Response(
            200, json={"choices": [{"message": {"role": "assistant", "content": "hello there"}}]}
        )

    result = await client_module.chat([{"role": "user", "content": "hi"}], transport=transport_for(handler))
    assert result.content == "hello there"
    assert result.tool_calls == []


async def test_chat_model_override_sent_in_payload():
    def handler(request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content)
        assert body["model"] == "gpt-4o-mini"
        return httpx.Response(200, json={"choices": [{"message": {"content": "ok"}}]})

    await client_module.chat(
        [{"role": "user", "content": "hi"}], model="gpt-4o-mini", transport=transport_for(handler)
    )


async def test_chat_sends_tools_in_openai_shape():
    def handler(request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content)
        assert body["tools"] == [
            {
                "type": "function",
                "function": {"name": "get_status", "description": "Gets status", "parameters": {"type": "object", "properties": {}}},
            }
        ]
        return httpx.Response(200, json={"choices": [{"message": {"content": "ok"}}]})

    await client_module.chat(
        [{"role": "user", "content": "hi"}],
        tools=[("get_status", "Gets status", {})],
        transport=transport_for(handler),
    )


async def test_chat_parses_tool_calls_from_response():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(
            200,
            json={
                "choices": [
                    {
                        "message": {
                            "role": "assistant",
                            "content": None,
                            "tool_calls": [
                                {
                                    "id": "call_1",
                                    "function": {"name": "track_order", "arguments": '{"order_id": "ORD-1001"}'},
                                }
                            ],
                        }
                    }
                ]
            },
        )

    result = await client_module.chat([{"role": "user", "content": "hi"}], transport=transport_for(handler))
    assert result.content == ""
    assert len(result.tool_calls) == 1
    assert result.tool_calls[0].name == "track_order"
    assert result.tool_calls[0].arguments == {"order_id": "ORD-1001"}


async def test_chat_malformed_tool_call_arguments_degrade_to_empty_dict():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(
            200,
            json={
                "choices": [
                    {
                        "message": {
                            "content": None,
                            "tool_calls": [{"id": "call_1", "function": {"name": "x", "arguments": "{not json"}}],
                        }
                    }
                ]
            },
        )

    result = await client_module.chat([{"role": "user", "content": "hi"}], transport=transport_for(handler))
    assert result.tool_calls[0].arguments == {}


async def test_chat_includes_auth_header_when_api_key_set(monkeypatch):
    monkeypatch.setattr(client_module, "API_KEY", "sk-test-123")

    def handler(request: httpx.Request) -> httpx.Response:
        assert request.headers["authorization"] == "Bearer sk-test-123"
        return httpx.Response(200, json={"choices": [{"message": {"content": "ok"}}]})

    await client_module.chat([{"role": "user", "content": "hi"}], transport=transport_for(handler))


async def test_chat_omits_auth_header_when_no_api_key(monkeypatch):
    monkeypatch.setattr(client_module, "API_KEY", "")

    def handler(request: httpx.Request) -> httpx.Response:
        assert "authorization" not in request.headers
        return httpx.Response(200, json={"choices": [{"message": {"content": "ok"}}]})

    await client_module.chat([{"role": "user", "content": "hi"}], transport=transport_for(handler))


async def test_chat_stream_yields_content_deltas():
    sse_body = (
        'data: {"choices": [{"delta": {"content": "Hel"}}]}\n\n'
        'data: {"choices": [{"delta": {"content": "lo"}}]}\n\n'
        "data: [DONE]\n\n"
    )

    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, content=sse_body, headers={"content-type": "text/event-stream"})

    chunks = [
        c async for c in client_module.chat_stream([{"role": "user", "content": "hi"}], transport=transport_for(handler))
    ]
    assert chunks == ["Hel", "lo"]


async def test_chat_stream_skips_deltas_with_no_content():
    sse_body = 'data: {"choices": [{"delta": {"role": "assistant"}}]}\n\ndata: [DONE]\n\n'

    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, content=sse_body, headers={"content-type": "text/event-stream"})

    chunks = [
        c async for c in client_module.chat_stream([{"role": "user", "content": "hi"}], transport=transport_for(handler))
    ]
    assert chunks == []
