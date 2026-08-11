from types import SimpleNamespace
from unittest.mock import AsyncMock

import pytest

import tools.assembly as assembly_module
from mcp_client.client import McpTool
from tools.assembly import ToolAssemblyError, assemble_tools


def make_core(*, profile_roles_allowed, profile_connector_ids, connectors):
    core = SimpleNamespace()
    core.bot_profile = SimpleNamespace(
        GetActiveBotProfile=AsyncMock(
            return_value=SimpleNamespace(
                bot_profile=SimpleNamespace(
                    name="external",
                    roles_allowed=profile_roles_allowed,
                    connector_ids=profile_connector_ids,
                )
            )
        )
    )
    core.connector = SimpleNamespace(ListConnectors=AsyncMock(return_value=SimpleNamespace(connectors=connectors)))
    return core


def make_connector(id_, name, endpoint="http://x.example", status="active"):
    return SimpleNamespace(_id=id_, name=name, endpoint=endpoint, status=status)


async def test_rejects_role_not_in_profiles_allowed(monkeypatch):
    core = make_core(profile_roles_allowed=[4], profile_connector_ids=[], connectors=[])  # 4 = customer
    with pytest.raises(ToolAssemblyError, match="not permitted"):
        await assemble_tools(core, tenant_id="tnt_1", channel="web-widget", role="staff", token="tok")


async def test_allows_role_in_profiles_allowed(monkeypatch):
    core = make_core(profile_roles_allowed=[4], profile_connector_ids=[], connectors=[])
    result = await assemble_tools(core, tenant_id="tnt_1", channel="web-widget", role="customer", token="tok")
    assert result == []


async def test_only_fetches_tools_for_profiles_connectors(monkeypatch):
    core = make_core(
        profile_roles_allowed=[4],
        profile_connector_ids=["conn_1"],
        connectors=[make_connector("conn_1", "booking-mcp"), make_connector("conn_2", "unrelated-mcp")],
    )

    async def fake_list_tools(endpoint):
        assert endpoint == "http://x.example"
        return [McpTool(name="book_appointment", description="Books a slot", input_schema={})]

    monkeypatch.setattr(assembly_module, "list_tools", fake_list_tools)

    result = await assemble_tools(core, tenant_id="tnt_1", channel="web-widget", role="customer", token="tok")
    assert len(result) == 1
    assert result[0].connector_name == "booking-mcp"
    assert result[0].tool.name == "book_appointment"


async def test_still_tries_a_connector_core_has_not_marked_active(monkeypatch):
    # Deliberate: connector.status reflects core's own cache validity, not
    # whether orchestrator can reach it right now — see assembly.py's
    # comment. A "pending" connector should still get a live tools/list
    # attempt, not be skipped outright.
    core = make_core(
        profile_roles_allowed=[4],
        profile_connector_ids=["conn_1"],
        connectors=[make_connector("conn_1", "pending-mcp", status="pending")],
    )

    async def fake_list_tools(endpoint):
        return [McpTool(name="ping", description="Pings the connector", input_schema={})]

    monkeypatch.setattr(assembly_module, "list_tools", fake_list_tools)

    result = await assemble_tools(core, tenant_id="tnt_1", channel="web-widget", role="customer", token="tok")
    assert len(result) == 1
    assert result[0].tool.name == "ping"


async def test_drops_unreachable_connector_without_failing_the_whole_request(monkeypatch):
    core = make_core(
        profile_roles_allowed=[4],
        profile_connector_ids=["conn_1"],
        connectors=[make_connector("conn_1", "down-mcp")],
    )

    async def failing_list_tools(endpoint):
        raise ConnectionError("connector is down")

    monkeypatch.setattr(assembly_module, "list_tools", failing_list_tools)

    result = await assemble_tools(core, tenant_id="tnt_1", channel="web-widget", role="customer", token="tok")
    assert result == []
