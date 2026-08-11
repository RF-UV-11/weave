"""Exercises mcp_client against the real reference-mcp server object
in-process (no network) — this is genuine MCP protocol traffic
(initialize -> tools/list -> tools/call), not a mock of our own client.
"""

import importlib.util
from pathlib import Path

import pytest

from mcp_client.client import MissingToolDescriptionError, call_tool, list_tools

_REFERENCE_MCP_PATH = Path(__file__).parent.parent.parent / "connectors" / "reference-mcp" / "server.py"


def _load_reference_mcp_module():
    # Loaded by explicit path under a distinct name, not a plain `import
    # server` — orchestrator has its own `server` package (chat_service.py
    # etc.), and a bare import would resolve to whichever one sys.path
    # happens to find first.
    spec = importlib.util.spec_from_file_location("reference_mcp_server", _REFERENCE_MCP_PATH)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


@pytest.fixture
def reference_server():
    module = _load_reference_mcp_module()
    return module.server


async def test_list_tools_returns_all_reference_tools(reference_server):
    tools = await list_tools(reference_server)
    names = {t.name for t in tools}
    assert names == {"book_appointment", "cancel_appointment", "list_appointments"}


async def test_list_tools_every_tool_has_a_description(reference_server):
    tools = await list_tools(reference_server)
    for t in tools:
        assert t.description.strip(), f"{t.name} has no description"


async def test_call_tool_books_an_appointment(reference_server):
    result = await call_tool(reference_server, "book_appointment", {"date": "2026-08-20", "time": "15:00", "customer_name": "Ada"})
    assert "Booked" in result
    assert "Ada" in result


async def test_call_tool_round_trip_book_then_list(reference_server):
    await call_tool(reference_server, "book_appointment", {"date": "2026-08-20", "time": "15:00", "customer_name": "Ada"})
    result = await call_tool(reference_server, "list_appointments", {})
    assert "Ada" in result


async def test_list_tools_rejects_tool_missing_description(monkeypatch):
    from mcp.server import MCPServer

    undescribed = MCPServer(name="undescribed")

    @undescribed.tool(description="")
    def no_description_tool(x: str) -> str:
        return x

    with pytest.raises(MissingToolDescriptionError):
        await list_tools(undescribed)
