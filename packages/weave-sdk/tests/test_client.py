import json
from types import SimpleNamespace
from unittest.mock import AsyncMock

import pytest

from weave.client import RegisteredTool, SyncWeaveClient, WeaveClient, connect, connect_async


def make_core_with_http_tool_stub(register_response=None, list_response=None, deregister_response=None):
    core = SimpleNamespace()
    core.http_tool = SimpleNamespace(
        RegisterHttpTool=AsyncMock(return_value=register_response),
        ListHttpTools=AsyncMock(return_value=list_response),
        DeregisterHttpTool=AsyncMock(return_value=deregister_response),
    )
    core.close = AsyncMock()
    return core


def fake_registered_tool(id_="htool_1", name="get_status", description="Gets status", endpoint="https://x", method="GET"):
    return SimpleNamespace(_id=id_, name=name, description=description, http_endpoint=endpoint, http_method=method)


async def test_add_tool_sends_json_encoded_schema_and_returns_registered_tool():
    core = make_core_with_http_tool_stub(
        register_response=SimpleNamespace(http_tool=fake_registered_tool())
    )
    client = WeaveClient(core, "tnt_1", "tok_abc")

    result = await client.add_tool(
        name="get_status",
        description="Gets status",
        endpoint="https://api.acme.test/orders/{id}/status",
        method="GET",
        params_schema={"type": "object", "properties": {"id": {"type": "string"}}},
    )

    assert isinstance(result, RegisteredTool)
    assert result.id == "htool_1"
    assert result.name == "get_status"

    call = core.http_tool.RegisterHttpTool.call_args
    req = call.args[0]
    assert req.tenant_id == "tnt_1"
    assert req.name == "get_status"
    assert req.http_endpoint == "https://api.acme.test/orders/{id}/status"
    assert json.loads(req.params_schema) == {"type": "object", "properties": {"id": {"type": "string"}}}
    # bearer_metadata should carry the client's token
    assert call.kwargs["metadata"] == [("authorization", "Bearer tok_abc")]


async def test_add_tool_defaults_schema_when_none_given():
    core = make_core_with_http_tool_stub(register_response=SimpleNamespace(http_tool=fake_registered_tool()))
    client = WeaveClient(core, "tnt_1", "tok_abc")

    await client.add_tool(name="x", description="does x", endpoint="https://x")

    req = core.http_tool.RegisterHttpTool.call_args.args[0]
    assert json.loads(req.params_schema) == {"type": "object", "properties": {}}


async def test_add_tool_passes_credential_secret():
    core = make_core_with_http_tool_stub(register_response=SimpleNamespace(http_tool=fake_registered_tool()))
    client = WeaveClient(core, "tnt_1", "tok_abc")

    await client.add_tool(name="x", description="does x", endpoint="https://x", credential_secret="sk-secret")

    req = core.http_tool.RegisterHttpTool.call_args.args[0]
    assert req.credential_secret == "sk-secret"


async def test_list_tools_returns_registered_tools():
    core = make_core_with_http_tool_stub(
        list_response=SimpleNamespace(http_tools=[fake_registered_tool(), fake_registered_tool(id_="htool_2", name="other")])
    )
    client = WeaveClient(core, "tnt_1", "tok_abc")

    tools = await client.list_tools()

    assert [t.name for t in tools] == ["get_status", "other"]
    assert core.http_tool.ListHttpTools.call_args.args[0].tenant_id == "tnt_1"


async def test_remove_tool_calls_deregister_with_metadata():
    core = make_core_with_http_tool_stub(deregister_response=SimpleNamespace())
    client = WeaveClient(core, "tnt_1", "tok_abc")

    await client.remove_tool("htool_1")

    call = core.http_tool.DeregisterHttpTool.call_args
    assert call.args[0].http_tool_id == "htool_1"
    assert call.kwargs["metadata"] == [("authorization", "Bearer tok_abc")]


async def test_connect_async_logs_in_and_returns_client(monkeypatch):
    import weave.client as client_module

    fake_core = make_core_with_http_tool_stub()
    fake_core.auth = SimpleNamespace(Login=AsyncMock(return_value=SimpleNamespace(access_token="tok_from_login")))

    monkeypatch.setattr(client_module, "CoreClient", lambda addr: fake_core)

    client = await connect_async(tenant_id="tnt_1", email="owner@x.test", password="hunter2hunter2")

    assert isinstance(client, WeaveClient)
    assert client._token == "tok_from_login"
    fake_core.auth.Login.assert_awaited_once()


async def test_connect_async_closes_core_on_login_failure(monkeypatch):
    import weave.client as client_module

    fake_core = make_core_with_http_tool_stub()
    fake_core.auth = SimpleNamespace(Login=AsyncMock(side_effect=RuntimeError("bad credentials")))

    monkeypatch.setattr(client_module, "CoreClient", lambda addr: fake_core)

    with pytest.raises(RuntimeError):
        await connect_async(tenant_id="tnt_1", email="owner@x.test", password="wrong")

    fake_core.close.assert_awaited_once()


def test_sync_connect_add_tool_and_close(monkeypatch):
    # End-to-end smoke test of the sync facade's persistent-event-loop
    # design — this is exactly the path that would break with a naive
    # `asyncio.run()`-per-call implementation (grpc.aio objects aren't
    # portable across event loops), so it's worth covering explicitly.
    import weave.client as client_module

    fake_core = make_core_with_http_tool_stub(register_response=SimpleNamespace(http_tool=fake_registered_tool()))
    fake_core.auth = SimpleNamespace(Login=AsyncMock(return_value=SimpleNamespace(access_token="tok_sync")))

    monkeypatch.setattr(client_module, "CoreClient", lambda addr: fake_core)

    client: SyncWeaveClient = connect(tenant_id="tnt_1", email="owner@x.test", password="hunter2hunter2")
    try:
        tool = client.add_tool(name="x", description="does x", endpoint="https://x")
        assert tool.name == "get_status"  # from fake_registered_tool()'s default

        tool2 = client.add_tool(name="y", description="does y", endpoint="https://y")
        assert tool2.id == "htool_1"
    finally:
        client.close()
