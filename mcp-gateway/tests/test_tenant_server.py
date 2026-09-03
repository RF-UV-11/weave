"""Exercises build_tenant_server via real MCP protocol (mcp.client.Client
accepts a lowlevel Server object directly, in-process, no network) —
this proves the actual wire behavior (initialize -> tools/list ->
tools/call), not just that the right Python functions get called.
"""

from types import SimpleNamespace
from unittest.mock import AsyncMock

import pytest
from mcp.client import Client

import gateway.tenant_server as tenant_server_module
from gateway.tenant_server import build_tenant_server
from gateway.http_caller import HttpToolCallError


def make_tool(name, description="does something", http_endpoint="https://x", http_method="GET",
              params_schema="", credential_ref_id="", _id="htool_1", visibility="external", category="general"):
    return SimpleNamespace(
        _id=_id, name=name, description=description, http_endpoint=http_endpoint,
        http_method=http_method, params_schema=params_schema, credential_ref_id=credential_ref_id,
        visibility=visibility, category=category,
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
        assert result.tools[0].meta == {"visibility": "internal", "category": "analytics"}


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

    async def fake_call_http_tool(*, endpoint, method, arguments, secret):
        captured.update(endpoint=endpoint, method=method, arguments=arguments, secret=secret)
        return "the result"

    monkeypatch.setattr(tenant_server_module, "call_http_tool", fake_call_http_tool)

    core = make_core([make_tool("do_thing", http_endpoint="https://x/do", http_method="POST")])
    server = build_tenant_server(core, "tnt_1")

    async with Client(server) as client:
        result = await client.call_tool("do_thing", {"a": 1})
        assert result.content[0].text == "the result"

    assert captured == {"endpoint": "https://x/do", "method": "POST", "arguments": {"a": 1}, "secret": None}


async def test_call_tool_reveals_and_passes_credential(monkeypatch):
    captured = {}

    async def fake_call_http_tool(*, endpoint, method, arguments, secret):
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


async def test_tenants_are_isolated_via_separate_core_calls():
    # build_tenant_server closes over tenant_id — confirms the tenant_id
    # actually reaches core's tenant-scoped ListHttpTools call.
    core = make_core([make_tool("only_for_this_tenant")])
    build_tenant_server(core, "tnt_specific")

    async with Client(build_tenant_server(core, "tnt_specific")) as client:
        await client.list_tools()

    call_args = core.http_tool.ListHttpTools.call_args
    assert call_args.args[0].tenant_id == "tnt_specific"
