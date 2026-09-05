import json
from types import SimpleNamespace
from unittest.mock import AsyncMock

import pytest

from weave.client import BotProfileHandle, RegisteredTool, SyncWeaveClient, WeaveClient, connect, connect_async, sign_up


def make_core_with_http_tool_stub(
    register_response=None, list_response=None, deregister_response=None,
    bot_profile_response=None, connectors_response=None,
):
    core = SimpleNamespace()
    core.http_tool = SimpleNamespace(
        RegisterHttpTool=AsyncMock(return_value=register_response),
        ListHttpTools=AsyncMock(return_value=list_response),
        DeregisterHttpTool=AsyncMock(return_value=deregister_response),
    )
    core.bot_profile = SimpleNamespace(CreateBotProfile=AsyncMock(return_value=bot_profile_response))
    core.connector = SimpleNamespace(
        ListConnectors=AsyncMock(
            return_value=connectors_response or SimpleNamespace(connectors=[])
        )
    )
    core.close = AsyncMock()
    return core


def fake_registered_tool(
    id_="htool_1", name="get_status", description="Gets status", endpoint="https://x", method="GET",
    visibility="internal", category="general", auth_mode="none",
):
    return SimpleNamespace(
        _id=id_, name=name, description=description, http_endpoint=endpoint, http_method=method,
        visibility=visibility, category=category, auth_mode=auth_mode,
    )


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


async def test_add_tool_passes_visibility_and_category():
    core = make_core_with_http_tool_stub(
        register_response=SimpleNamespace(http_tool=fake_registered_tool(visibility="external", category="analytics"))
    )
    client = WeaveClient(core, "tnt_1", "tok_abc")

    result = await client.add_tool(
        name="sales_report", description="Revenue report", endpoint="https://x",
        visibility="external", category="analytics",
    )

    req = core.http_tool.RegisterHttpTool.call_args.args[0]
    assert req.visibility == "external"
    assert req.category == "analytics"
    assert result.visibility == "external"
    assert result.category == "analytics"


async def test_add_tool_defaults_visibility_internal_and_category_general():
    core = make_core_with_http_tool_stub(register_response=SimpleNamespace(http_tool=fake_registered_tool()))
    client = WeaveClient(core, "tnt_1", "tok_abc")

    await client.add_tool(name="x", description="does x", endpoint="https://x")

    req = core.http_tool.RegisterHttpTool.call_args.args[0]
    assert req.visibility == "internal"
    assert req.category == "general"


async def test_add_tool_defaults_auth_mode_to_none():
    core = make_core_with_http_tool_stub(register_response=SimpleNamespace(http_tool=fake_registered_tool()))
    client = WeaveClient(core, "tnt_1", "tok_abc")

    result = await client.add_tool(name="x", description="does x", endpoint="https://x")

    req = core.http_tool.RegisterHttpTool.call_args.args[0]
    assert req.auth_mode == "none"
    assert result.auth_mode == "none"


async def test_add_tool_passes_user_token_auth_mode():
    core = make_core_with_http_tool_stub(
        register_response=SimpleNamespace(http_tool=fake_registered_tool(auth_mode="user_token"))
    )
    client = WeaveClient(core, "tnt_1", "tok_abc")

    result = await client.add_tool(
        name="get_my_transactions", description="the caller's own transactions", endpoint="https://x",
        credential_secret="sk-signing-key", auth_mode="user_token",
    )

    req = core.http_tool.RegisterHttpTool.call_args.args[0]
    assert req.auth_mode == "user_token"
    assert req.credential_secret == "sk-signing-key"
    assert result.auth_mode == "user_token"


async def test_create_bot_profile_resolves_managed_connector_by_default():
    core = make_core_with_http_tool_stub(
        bot_profile_response=SimpleNamespace(
            bot_profile=SimpleNamespace(_id="profile_1", name="external", visibility="external")
        ),
        connectors_response=SimpleNamespace(
            connectors=[SimpleNamespace(_id="conn_1", name="weave_managed")]
        ),
    )
    client = WeaveClient(core, "tnt_1", "tok_abc")

    result = await client.create_bot_profile(
        name="external", channels=["web-widget"], roles_allowed=["customer"], visibility="external",
    )

    assert isinstance(result, BotProfileHandle)
    assert result.id == "profile_1"
    req = core.bot_profile.CreateBotProfile.call_args.args[0]
    assert req.connector_ids == ["conn_1"]
    assert req.roles_allowed == [4]
    assert req.visibility == "external"


async def test_create_bot_profile_passes_web_search_enabled_and_guardrails():
    core = make_core_with_http_tool_stub(
        bot_profile_response=SimpleNamespace(
            bot_profile=SimpleNamespace(_id="profile_1", name="external", visibility="external")
        ),
    )
    client = WeaveClient(core, "tnt_1", "tok_abc")

    await client.create_bot_profile(
        name="external", channels=["web-widget"], roles_allowed=["customer"],
        web_search_enabled=True, guardrails=["Never disclose supplier names."],
    )

    req = core.bot_profile.CreateBotProfile.call_args.args[0]
    assert req.web_search_enabled is True
    assert list(req.guardrails) == ["Never disclose supplier names."]


async def test_remove_tool_calls_deregister_with_metadata():
    core = make_core_with_http_tool_stub(deregister_response=SimpleNamespace())
    client = WeaveClient(core, "tnt_1", "tok_abc")

    await client.remove_tool("htool_1")

    call = core.http_tool.DeregisterHttpTool.call_args
    assert call.args[0].http_tool_id == "htool_1"
    assert call.kwargs["metadata"] == [("authorization", "Bearer tok_abc")]


async def test_sign_up_creates_tenant_and_registers_owner(monkeypatch):
    import weave.client as client_module

    fake_core = make_core_with_http_tool_stub()
    fake_core.tenant = SimpleNamespace(
        CreateTenant=AsyncMock(return_value=SimpleNamespace(tenant=SimpleNamespace(_id="tnt_new")))
    )
    fake_core.auth = SimpleNamespace(Register=AsyncMock(return_value=SimpleNamespace()))

    monkeypatch.setattr(client_module, "CoreClient", lambda addr: fake_core)

    tenant_id = await sign_up(display_name="Tarang Electronics", email="owner@tarang.test", password="hunter2hunter2")

    assert tenant_id == "tnt_new"
    create_req = fake_core.tenant.CreateTenant.call_args.args[0]
    assert create_req.display_name == "Tarang Electronics"
    assert create_req.tenant_type == "business"
    register_req = fake_core.auth.Register.call_args.args[0]
    assert register_req.tenant_id == "tnt_new"
    assert register_req.email == "owner@tarang.test"
    assert register_req.role == 1  # owner
    fake_core.close.assert_awaited_once()


async def test_sign_up_closes_core_on_failure(monkeypatch):
    import weave.client as client_module

    fake_core = make_core_with_http_tool_stub()
    fake_core.tenant = SimpleNamespace(CreateTenant=AsyncMock(side_effect=RuntimeError("core unreachable")))

    monkeypatch.setattr(client_module, "CoreClient", lambda addr: fake_core)

    with pytest.raises(RuntimeError):
        await sign_up(display_name="X", email="owner@x.test", password="hunter2hunter2")

    fake_core.close.assert_awaited_once()


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
