import httpx
import pytest

from gateway.http_caller import HttpToolCallError, call_http_tool


def transport_for(handler):
    return httpx.MockTransport(handler)


async def test_substitutes_path_parameters():
    captured = {}

    def handler(request: httpx.Request) -> httpx.Response:
        captured["url"] = str(request.url)
        return httpx.Response(200, text="ok")

    result = await call_http_tool(
        endpoint="https://api.acme.test/orders/{order_id}/status",
        method="GET",
        arguments={"order_id": "abc123"},
        secret=None,
        transport=transport_for(handler),
    )
    assert result == "ok"
    assert "/orders/abc123/status" in captured["url"]
    assert "order_id" not in captured["url"]


async def test_missing_path_parameter_raises():
    with pytest.raises(HttpToolCallError, match="order_id"):
        await call_http_tool(
            endpoint="https://api.acme.test/orders/{order_id}/status",
            method="GET",
            arguments={},
            secret=None,
            transport=transport_for(lambda r: httpx.Response(200)),
        )


async def test_get_uses_query_params():
    captured = {}

    def handler(request: httpx.Request) -> httpx.Response:
        captured["query"] = dict(request.url.params)
        return httpx.Response(200, text="ok")

    await call_http_tool(
        endpoint="https://api.acme.test/search",
        method="GET",
        arguments={"q": "widgets"},
        secret=None,
        transport=transport_for(handler),
    )
    assert captured["query"] == {"q": "widgets"}


async def test_post_uses_json_body():
    captured = {}

    def handler(request: httpx.Request) -> httpx.Response:
        captured["body"] = request.content
        return httpx.Response(200, text="created")

    await call_http_tool(
        endpoint="https://api.acme.test/orders",
        method="POST",
        arguments={"item": "widget", "qty": 3},
        secret=None,
        transport=transport_for(handler),
    )
    import json

    assert json.loads(captured["body"]) == {"item": "widget", "qty": 3}


async def test_injects_credential_as_bearer_header():
    captured = {}

    def handler(request: httpx.Request) -> httpx.Response:
        captured["auth"] = request.headers.get("authorization")
        return httpx.Response(200, text="ok")

    await call_http_tool(
        endpoint="https://api.acme.test/secure",
        method="GET",
        arguments={},
        secret="sk-secret-token",
        transport=transport_for(handler),
    )
    assert captured["auth"] == "Bearer sk-secret-token"


async def test_no_credential_means_no_auth_header():
    captured = {}

    def handler(request: httpx.Request) -> httpx.Response:
        captured["auth"] = request.headers.get("authorization")
        return httpx.Response(200, text="ok")

    await call_http_tool(
        endpoint="https://api.acme.test/public",
        method="GET",
        arguments={},
        secret=None,
        transport=transport_for(handler),
    )
    assert captured["auth"] is None


async def test_non_2xx_response_returned_as_content_not_raised():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(404, text="order not found")

    result = await call_http_tool(
        endpoint="https://api.acme.test/orders/999",
        method="GET",
        arguments={},
        secret=None,
        transport=transport_for(handler),
    )
    assert result == "HTTP 404: order not found"


async def test_response_exceeding_size_cap_raises():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, content=b"x" * (1_000_001))

    with pytest.raises(HttpToolCallError, match="byte limit"):
        await call_http_tool(
            endpoint="https://api.acme.test/huge",
            method="GET",
            arguments={},
            secret=None,
            transport=transport_for(handler),
        )


async def test_connection_error_raises_tool_call_error():
    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("connection refused")

    with pytest.raises(HttpToolCallError, match="failed"):
        await call_http_tool(
            endpoint="https://api.acme.test/down",
            method="GET",
            arguments={},
            secret=None,
            transport=transport_for(handler),
        )
