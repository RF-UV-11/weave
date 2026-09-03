"""Thin wrapper around the official `mcp` SDK's Client: initialize ->
tools/list -> tools/call against a single connector endpoint.

This is orchestrator's *real* MCP client — unlike core/mcpclient (Go),
which only ever speaks a minimal JSON-RPC "tools/list" to cache a
manifest, this one does the full MCP handshake and actually calls tools,
per docs/architecture/ARCHITECTURE.md §3.
"""

from dataclasses import dataclass
from typing import Any, TypeAlias

from mcp.client import Client
from mcp.server import MCPServer

# Normally a URL string (a real network connector); MCPServer accepted
# too so tests can talk to connectors/reference-mcp in-process, with no
# network involved — see tests/test_mcp_client.py.
McpTarget: TypeAlias = str | MCPServer


@dataclass
class McpTool:
    name: str
    description: str
    input_schema: dict[str, Any]
    # Weave-specific extension metadata carried in MCP's standard `_meta`
    # field (mcp-gateway/gateway/tenant_server.py). A real third-party MCP
    # server won't set these, so absence defaults to the least
    # restrictive values: every profile can see the tool, and it's not
    # singled out as analytics-flavored.
    visibility: str = "external"
    category: str = "general"


class MissingToolDescriptionError(ValueError):
    """Raised when a connector's tools/list includes a tool with no
    description — mandatory context, not optional metadata, same
    requirement core/mcpclient enforces at registration time (PLAN.md's
    Phase 3 tool-description item, docs/architecture/ARCHITECTURE.md §3)."""


async def list_tools(endpoint: McpTarget) -> list[McpTool]:
    """Connects to endpoint, performs the MCP initialize handshake, and
    returns every tool it exposes. Raises MissingToolDescriptionError if
    any tool has no description."""
    async with Client(endpoint) as client:
        result = await client.list_tools()

    tools: list[McpTool] = []
    missing: list[str] = []
    for t in result.tools:
        if not (t.description or "").strip():
            missing.append(t.name)
            continue
        meta = t.meta or {}
        tools.append(
            McpTool(
                name=t.name,
                description=t.description,
                input_schema=t.input_schema or {},
                visibility=meta.get("visibility") or "external",
                category=meta.get("category") or "general",
            )
        )
    if missing:
        raise MissingToolDescriptionError(f"connector at {endpoint} has tools missing a description: {missing}")
    return tools


async def call_tool(endpoint: McpTarget, tool_name: str, arguments: dict[str, Any]) -> str:
    """Calls tool_name on the connector at endpoint and returns its
    result as text (MCP tool results are a list of content blocks; this
    concatenates any text blocks, which covers the reference connector
    and any well-behaved tool)."""
    async with Client(endpoint) as client:
        result = await client.call_tool(tool_name, arguments)

    parts = [block.text for block in result.content if getattr(block, "type", None) == "text"]
    return "\n".join(parts) if parts else ""
