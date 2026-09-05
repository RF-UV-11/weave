"""Exercises build_tenant_server via real MCP protocol (mcp.client.Client
accepts a lowlevel Server object directly, in-process, no network) —
this proves the actual wire behavior (initialize -> tools/list ->
tools/call), not just that the right Python functions get called.
"""

import time
from types import SimpleNamespace
from unittest.mock import AsyncMock

import jwt
import pytest
from mcp.client import Client

import gateway.tenant_server as tenant_server_module
import gateway.user_assertion as user_assertion_module
from gateway.tenant_server import build_tenant_server
from gateway.http_caller import HttpToolCallError
from gateway.http_signing import sign_user_identity

JWT_SECRET = "test-secret-not-for-prod-but-long-enough-for-hs256"


@pytest.fixture(autouse=True)
def _jwt_secret(monkeypatch):
    monkeypatch.setattr(user_assertion_module, "JWT_SECRET", JWT_SECRET)


def make_assertion(*, tenant_id="tnt_1", user_id="usr_1", **overrides):
    now = int(time.time())
    payload = {"typ": "user_assertion", "tenant_id": tenant_id, "user_id": user_id, "iat": now, "exp": now + 60}
    payload.update(overrides)
    return jwt.encode(payload, JWT_SECRET, algorithm="HS256")


def make_tool(name, description="does something", http_endpoint="https://x", http_method="GET",
              params_schema="", credential_ref_id="", _id="htool_1", visibility="external", category="general",
              auth_mode="none"):
    return SimpleNamespace(
        _id=_id, name=name, description=description, http_endpoint=http_endpoint,
        http_method=http_method, params_schema=params_schema, credential_ref_id=credential_ref_id,
        visibility=visibility, category=category, auth_mode=auth_mode,
    )


def make_core(tools, secret=None):
    core = SimpleNamespace()
    core.http_tool = SimpleNamespace(
        ListHttpTools=AsyncMock(return_value=SimpleNamespace(http_tools=tools)),
        RevealHttpToolCredential=AsyncMock(return_value=SimpleNamespace(secret=secret or "")),
    )
    return core


async def test_list_tools_returns_registered_tools():
    core = make_core([make_tool("get_status", description="Gets order status")])
    server = build_tenant_server(core, "tnt_1")

    async with Client(server) as client:
        result = await client.list_tools()
        names = {t.name for t in result.tools}
        assert names == {"get_status"}
        assert result.tools[0].description == "Gets order status"


async def test_list_tools_carries_visibility_and_category_in_meta():
    core = make_core([make_tool("sales_report", visibility="internal", category="analytics")])
    server = build_tenant_server(core, "tnt_1")

    async with Client(server) as client:
        result = await client.list_tools()
        assert result.tools[0].meta == {"visibility": "internal", "category": "analytics", "auth_mode": "none"}


async def test_list_tools_carries_auth_mode_in_meta():
    core = make_core([make_tool("get_my_transactions", auth_mode="user_token")])
    server = build_tenant_server(core, "tnt_1")

    async with Client(server) as client:
        result = await client.list_tools()
        assert result.tools[0].meta["auth_mode"] == "user_token"


async def test_list_tools_empty_for_tenant_with_no_tools():
    core = make_core([])
    server = build_tenant_server(core, "tnt_1")

    async with Client(server) as client:
        result = await client.list_tools()
        assert result.tools == []


async def test_list_tools_uses_stored_params_schema():
    core = make_core([make_tool("search", params_schema='{"type":"object","properties":{"q":{"type":"string"}}}')])
    server = build_tenant_server(core, "tnt_1")

    async with Client(server) as client:
        result = await client.list_tools()
        assert result.tools[0].input_schema == {"type": "object", "properties": {"q": {"type": "string"}}}


async def test_list_tools_malformed_schema_defaults_to_empty_object():
    core = make_core([make_tool("bad_schema", params_schema="not json")])
    server = build_tenant_server(core, "tnt_1")

    async with Client(server) as client:
        result = await client.list_tools()
        assert result.tools[0].input_schema == {"type": "object", "properties": {}}


async def test_call_tool_invokes_http_caller_with_resolved_args(monkeypatch):
    captured = {}

    async def fake_call_http_tool(*, endpoint, method, arguments, secret, extra_headers=None):
        captured.update(endpoint=endpoint, method=method, arguments=arguments, secret=secret, extra_headers=extra_headers)
        return "the result"

    monkeypatch.setattr(tenant_server_module, "call_http_tool", fake_call_http_tool)

    core = make_core([make_tool("do_thing", http_endpoint="https://x/do", http_method="POST")])
    server = build_tenant_server(core, "tnt_1")

    async with Client(server) as client:
        result = await client.call_tool("do_thing", {"a": 1})
        assert result.content[0].text == "the result"

    assert captured == {
        "endpoint": "https://x/do", "method": "POST", "arguments": {"a": 1}, "secret": None, "extra_headers": {},
    }


async def test_call_tool_reveals_and_passes_credential(monkeypatch):
    captured = {}

    async def fake_call_http_tool(*, endpoint, method, arguments, secret, extra_headers=None):
        captured["secret"] = secret
        return "ok"

    monkeypatch.setattr(tenant_server_module, "call_http_tool", fake_call_http_tool)

    core = make_core([make_tool("secure_thing", credential_ref_id="cred_1")], secret="sk-revealed")
    server = build_tenant_server(core, "tnt_1")

    async with Client(server) as client:
        await client.call_tool("secure_thing", {})

    assert captured["secret"] == "sk-revealed"
    core.http_tool.RevealHttpToolCredential.assert_awaited_once()


async def test_call_tool_unknown_name_returns_error_result():
    core = make_core([make_tool("known_tool")])
    server = build_tenant_server(core, "tnt_1")

    async with Client(server, raise_exceptions=False) as client:
        result = await client.call_tool("unknown_tool", {})
        assert result.is_error
        assert "not registered" in result.content[0].text


async def test_call_tool_propagates_caller_error_as_error_result(monkeypatch):
    async def failing_call_http_tool(**kwargs):
        raise HttpToolCallError("endpoint is down")

    monkeypatch.setattr(tenant_server_module, "call_http_tool", failing_call_http_tool)

    core = make_core([make_tool("flaky_tool")])
    server = build_tenant_server(core, "tnt_1")

    async with Client(server, raise_exceptions=False) as client:
        result = await client.call_tool("flaky_tool", {})
        assert result.is_error
        assert "endpoint is down" in result.content[0].text


async def test_call_tool_user_token_without_assertion_is_rejected():
    core = make_core([make_tool("get_my_transactions", auth_mode="user_token", credential_ref_id="cred_1")])
    server = build_tenant_server(core, "tnt_1")

    async with Client(server, raise_exceptions=False) as client:
        result = await client.call_tool("get_my_transactions", {})
        assert result.is_error
        assert "verified end-user identity" in result.content[0].text


async def test_call_tool_user_token_with_invalid_assertion_is_rejected():
    core = make_core([make_tool("get_my_transactions", auth_mode="user_token", credential_ref_id="cred_1")])
    server = build_tenant_server(core, "tnt_1")

    async with Client(server, raise_exceptions=False) as client:
        result = await client.call_tool("get_my_transactions", {}, meta={"weave_user_assertion": "not-a-real-jwt"})
        assert result.is_error
        assert "invalid end-user identity" in result.content[0].text


async def test_call_tool_user_token_for_wrong_tenant_is_rejected():
    # A valid assertion, but minted for a different tenant than this MCP
    # server instance is scoped to — must be rejected even though the
    # signature itself is valid (defense in depth, see
    # user_assertion.py's verify_user_assertion).
    core = make_core([make_tool("get_my_transactions", auth_mode="user_token", credential_ref_id="cred_1")])
    server = build_tenant_server(core, "tnt_1")
    assertion = make_assertion(tenant_id="tnt_OTHER")

    async with Client(server, raise_exceptions=False) as client:
        result = await client.call_tool("get_my_transactions", {}, meta={"weave_user_assertion": assertion})
        assert result.is_error
        assert "invalid end-user identity" in result.content[0].text


async def test_call_tool_user_token_without_credential_configured_is_rejected():
    core = make_core([make_tool("get_my_transactions", auth_mode="user_token", credential_ref_id="")])
    server = build_tenant_server(core, "tnt_1")
    assertion = make_assertion()

    async with Client(server, raise_exceptions=False) as client:
        result = await client.call_tool("get_my_transactions", {}, meta={"weave_user_assertion": assertion})
        assert result.is_error
        assert "misconfigured" in result.content[0].text


async def test_call_tool_user_token_signs_and_forwards_verified_user_identity(monkeypatch):
    captured = {}

    async def fake_call_http_tool(*, endpoint, method, arguments, secret, extra_headers=None):
        captured.update(secret=secret, extra_headers=extra_headers)
        return "the user's own transactions"

    monkeypatch.setattr(tenant_server_module, "call_http_tool", fake_call_http_tool)

    core = make_core(
        [make_tool("get_my_transactions", auth_mode="user_token", credential_ref_id="cred_1")],
        secret="shared-signing-key",
    )
    server = build_tenant_server(core, "tnt_1")
    assertion = make_assertion(tenant_id="tnt_1", user_id="usr_42")

    async with Client(server) as client:
        result = await client.call_tool("get_my_transactions", {}, meta={"weave_user_assertion": assertion})
        assert result.content[0].text == "the user's own transactions"

    # Never sent as a static bearer token in this mode.
    assert captured["secret"] is None
    assert captured["extra_headers"] == {
        "X-Weave-User-Id": "usr_42",
        "X-Weave-Tenant-Id": "tnt_1",
        "X-Weave-User-Signature": sign_user_identity("shared-signing-key", tenant_id="tnt_1", user_id="usr_42"),
    }


async def test_call_tool_auth_mode_none_never_verifies_or_forwards_user_identity(monkeypatch):
    # A tool that never opted into auth_mode="user_token" must keep
    # working exactly as before, even if a (harmless, always-sent)
    # assertion happens to be present in _meta.
    captured = {}

    async def fake_call_http_tool(*, endpoint, method, arguments, secret, extra_headers=None):
        captured.update(secret=secret, extra_headers=extra_headers)
        return "ok"

    monkeypatch.setattr(tenant_server_module, "call_http_tool", fake_call_http_tool)

    core = make_core([make_tool("track_order", credential_ref_id="cred_1")], secret="sk-static")
    server = build_tenant_server(core, "tnt_1")
    assertion = make_assertion()

    async with Client(server) as client:
        await client.call_tool("track_order", {}, meta={"weave_user_assertion": assertion})

    assert captured["secret"] == "sk-static"
    assert captured["extra_headers"] == {}


async def test_tenants_are_isolated_via_separate_core_calls():
    # build_tenant_server closes over tenant_id — confirms the tenant_id
    # actually reaches core's tenant-scoped ListHttpTools call.
    core = make_core([make_tool("only_for_this_tenant")])
    build_tenant_server(core, "tnt_specific")

    async with Client(build_tenant_server(core, "tnt_specific")) as client:
        await client.list_tools()

    call_args = core.http_tool.ListHttpTools.call_args
    assert call_args.args[0].tenant_id == "tnt_specific"
