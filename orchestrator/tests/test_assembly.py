from types import SimpleNamespace
from unittest.mock import AsyncMock

import pytest

import tools.assembly as assembly_module
from mcp_client.client import McpTool
from tools.assembly import ToolAssemblyError, assemble_tools


def make_core(
    *,
    profile_roles_allowed,
    profile_connector_ids,
    connectors,
    visibility="internal",
    guardrails=None,
    web_search_enabled=False,
):
    core = SimpleNamespace()
    core.bot_profile = SimpleNamespace(
        GetActiveBotProfile=AsyncMock(
            return_value=SimpleNamespace(
                bot_profile=SimpleNamespace(
                    name="external",
                    roles_allowed=profile_roles_allowed,
                    connector_ids=profile_connector_ids,
                    visibility=visibility,
                    guardrails=guardrails or [],
                    web_search_enabled=web_search_enabled,
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
    assert result.tools == []


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
    assert len(result.tools) == 1
    assert result.tools[0].connector_name == "booking-mcp"
    assert result.tools[0].tool.name == "book_appointment"


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
    assert len(result.tools) == 1
    assert result.tools[0].tool.name == "ping"


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
    assert result.tools == []


async def test_guardrails_active_only_when_external_with_rules():
    core = make_core(profile_roles_allowed=[4], profile_connector_ids=[], connectors=[], visibility="external", guardrails=["rule"])
    result = await assemble_tools(core, tenant_id="tnt_1", channel="web-widget", role="customer", token="tok")
    assert result.guardrails_active is True
    assert result.guardrails == ["rule"]


async def test_guardrails_inactive_for_internal_profile_even_with_rules():
    core = make_core(profile_roles_allowed=[4], profile_connector_ids=[], connectors=[], visibility="internal", guardrails=["rule"])
    result = await assemble_tools(core, tenant_id="tnt_1", channel="web-widget", role="customer", token="tok")
    assert result.guardrails_active is False


async def test_guardrails_inactive_for_external_profile_with_no_rules():
    core = make_core(profile_roles_allowed=[4], profile_connector_ids=[], connectors=[], visibility="external", guardrails=[])
    result = await assemble_tools(core, tenant_id="tnt_1", channel="web-widget", role="customer", token="tok")
    assert result.guardrails_active is False


async def test_web_search_enabled_propagates_from_profile(monkeypatch):
    core = make_core(
        profile_roles_allowed=[4], profile_connector_ids=[], connectors=[], web_search_enabled=True
    )
    result = await assemble_tools(core, tenant_id="tnt_1", channel="web-widget", role="customer", token="tok")
    assert result.web_search_enabled is True


async def test_web_search_disabled_by_default(monkeypatch):
    core = make_core(profile_roles_allowed=[4], profile_connector_ids=[], connectors=[])
    result = await assemble_tools(core, tenant_id="tnt_1", channel="web-widget", role="customer", token="tok")
    assert result.web_search_enabled is False


async def test_external_profile_only_sees_external_tools(monkeypatch):
    core = make_core(
        profile_roles_allowed=[4],
        profile_connector_ids=["conn_1"],
        connectors=[make_connector("conn_1", "acme-api")],
        visibility="external",
    )

    async def fake_list_tools(endpoint):
        return [
            McpTool(name="track_order", description="Track an order", input_schema={}, visibility="external"),
            McpTool(name="get_customer_pii", description="Internal only", input_schema={}, visibility="internal"),
        ]

    monkeypatch.setattr(assembly_module, "list_tools", fake_list_tools)

    result = await assemble_tools(core, tenant_id="tnt_1", channel="web-widget", role="customer", token="tok")
    names = {t.tool.name for t in result.tools}
    assert names == {"track_order"}


async def test_internal_profile_sees_every_tool_regardless_of_visibility(monkeypatch):
    core = make_core(
        profile_roles_allowed=[4],
        profile_connector_ids=["conn_1"],
        connectors=[make_connector("conn_1", "acme-api")],
        visibility="internal",
    )

    async def fake_list_tools(endpoint):
        return [
            McpTool(name="track_order", description="Track an order", input_schema={}, visibility="external"),
            McpTool(name="get_customer_pii", description="Internal only", input_schema={}, visibility="internal"),
        ]

    monkeypatch.setattr(assembly_module, "list_tools", fake_list_tools)

    result = await assemble_tools(core, tenant_id="tnt_1", channel="web-widget", role="customer", token="tok")
    names = {t.tool.name for t in result.tools}
    assert names == {"track_order", "get_customer_pii"}


async def test_analytics_tools_property_filters_by_category(monkeypatch):
    core = make_core(
        profile_roles_allowed=[4],
        profile_connector_ids=["conn_1"],
        connectors=[make_connector("conn_1", "acme-api")],
        visibility="internal",
    )

    async def fake_list_tools(endpoint):
        return [
            McpTool(name="track_order", description="Track an order", input_schema={}, category="general"),
            McpTool(name="sales_report", description="Revenue over time", input_schema={}, category="analytics"),
        ]

    monkeypatch.setattr(assembly_module, "list_tools", fake_list_tools)

    result = await assemble_tools(core, tenant_id="tnt_1", channel="web-widget", role="customer", token="tok")
    assert [t.tool.name for t in result.analytics_tools] == ["sales_report"]
