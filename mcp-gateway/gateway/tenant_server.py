"""Builds a real MCP server for one tenant, on demand, from their
registered HttpTools (core.HttpToolService) — this is what turns "a
business described a REST API" into genuine MCP protocol on the wire, so
orchestrator's MCP client (and anything else speaking real MCP) never
has to know these tools aren't backed by a real MCP server the business
runs themselves.

No caching: every call rebuilds from core's current HttpTool list. This
is deliberate for correctness (a tool registered a moment ago must be
usable immediately) at the cost of a core round trip per request — a
known, acceptable tradeoff for now; a short-TTL cache is a straightforward
follow-up if the extra latency ever matters.
"""

import json
import logging

from mcp import types
from mcp.server.lowlevel import Server

from weave_shared_clients import CoreClient
from core.data_access.v1 import http_tool_pb2

from .http_caller import HttpToolCallError, call_http_tool

logger = logging.getLogger("mcp_gateway.tenant_server")

_EMPTY_SCHEMA = {"type": "object", "properties": {}}


def _parse_schema(raw: str) -> dict:
    if not raw:
        return _EMPTY_SCHEMA
    try:
        parsed = json.loads(raw)
    except json.JSONDecodeError:
        logger.warning("malformed params_schema, defaulting to empty schema: %r", raw)
        return _EMPTY_SCHEMA
    return parsed if isinstance(parsed, dict) else _EMPTY_SCHEMA


async def _fetch_tools(core: CoreClient, tenant_id: str) -> list:
    resp = await core.http_tool.ListHttpTools(http_tool_pb2.ListHttpToolsRequest(tenant_id=tenant_id))
    return list(resp.http_tools)


async def _reveal_secret(core: CoreClient, tenant_id: str, http_tool_id: str) -> str | None:
    resp = await core.http_tool.RevealHttpToolCredential(
        http_tool_pb2.RevealHttpToolCredentialRequest(tenant_id=tenant_id, http_tool_id=http_tool_id)
    )
    return resp.secret or None


def build_tenant_server(core: CoreClient, tenant_id: str) -> Server:
    async def on_list_tools(ctx, params) -> types.ListToolsResult:
        tools = await _fetch_tools(core, tenant_id)
        return types.ListToolsResult(
            tools=[
                types.Tool(
                    name=t.name,
                    description=t.description,
                    inputSchema=_parse_schema(t.params_schema),
                    # Weave-specific extension metadata (docs/architecture/
                    # ARCHITECTURE.md §3): a real, non-Weave MCP server
                    # would never set these, so orchestrator's tool
                    # assembly treats their absence as "external"/"general"
                    # (the least restrictive default) rather than failing
                    # closed on tools that predate this convention.
                    _meta={
                        "visibility": t.visibility or "internal",
                        "category": t.category or "general",
                    },
                )
                for t in tools
            ]
        )

    async def on_call_tool(ctx, params: types.CallToolRequestParams) -> types.CallToolResult:
        name = params.name
        arguments = dict(params.arguments or {})

        tools = await _fetch_tools(core, tenant_id)
        match = next((t for t in tools if t.name == name), None)
        if match is None:
            return types.CallToolResult(
                content=[types.TextContent(type="text", text=f"Tool {name!r} is not registered for this tenant.")],
                isError=True,
            )

        secret = None
        if match.credential_ref_id:
            secret = await _reveal_secret(core, tenant_id, match._id)

        try:
            result_text = await call_http_tool(
                endpoint=match.http_endpoint,
                method=match.http_method,
                arguments=arguments,
                secret=secret,
            )
        except HttpToolCallError as exc:
            return types.CallToolResult(
                content=[types.TextContent(type="text", text=f"Tool call failed: {exc}")],
                isError=True,
            )

        return types.CallToolResult(content=[types.TextContent(type="text", text=result_text)])

    return Server(name=f"weave-managed-{tenant_id}", on_list_tools=on_list_tools, on_call_tool=on_call_tool)
