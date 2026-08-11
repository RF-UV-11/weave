import pytest

import server.graph as graph_module
from llm.ollama_client import ChatResult, ToolCall
from mcp_client.client import McpTool
from tools.assembly import AssembledTool


def make_tool(name="book_appointment", description="Book a slot"):
    return AssembledTool(
        connector_id="conn_1",
        connector_name="booking-mcp",
        endpoint="http://booking.example",
        tool=McpTool(name=name, description=description, input_schema={"type": "object", "properties": {}}),
    )


async def test_no_tools_available_never_calls_the_model(monkeypatch):
    async def boom(*args, **kwargs):
        raise AssertionError("chat() should not be called when there are no tools")

    monkeypatch.setattr(graph_module, "chat", boom)

    state = await graph_module.run_turn([{"role": "user", "content": "hi"}], [])
    assert state["tool_used"] == ""
    assert state["messages"] == [{"role": "user", "content": "hi"}]


async def test_model_answers_directly_without_a_tool(monkeypatch):
    async def fake_chat(messages, tools=None):
        return ChatResult(content="some answer the graph should discard", tool_calls=[])

    monkeypatch.setattr(graph_module, "chat", fake_chat)

    messages = [{"role": "user", "content": "what's 2+2?"}]
    state = await graph_module.run_turn(messages, [make_tool()])

    assert state["tool_used"] == ""
    assert state["connector_used"] == ""
    # Discarded on purpose (see graph.py) — messages unchanged, chat_service
    # does the real streaming synthesis call itself.
    assert state["messages"] == messages


async def test_tool_call_executes_and_embeds_description(monkeypatch):
    async def fake_chat(messages, tools=None):
        return ChatResult(content="", tool_calls=[ToolCall(name="book_appointment", arguments={"date": "2026-08-20"})])

    async def fake_call_tool(endpoint, name, arguments):
        assert endpoint == "http://booking.example"
        assert name == "book_appointment"
        return "Booked bkg_1"

    monkeypatch.setattr(graph_module, "chat", fake_chat)
    monkeypatch.setattr(graph_module, "call_tool", fake_call_tool)

    messages = [{"role": "user", "content": "book me a slot"}]
    state = await graph_module.run_turn(messages, [make_tool(description="Books an appointment slot")])

    assert state["tool_used"] == "book_appointment"
    assert state["connector_used"] == "booking-mcp"
    tool_message = state["messages"][-1]
    assert tool_message["role"] == "tool"
    assert "Books an appointment slot" in tool_message["content"]
    assert "Booked bkg_1" in tool_message["content"]


async def test_hallucinated_tool_name_handled_without_crashing(monkeypatch):
    async def fake_chat(messages, tools=None):
        return ChatResult(content="", tool_calls=[ToolCall(name="delete_universe", arguments={})])

    async def unexpected_call(*args, **kwargs):
        raise AssertionError("call_tool should not be invoked for an unknown tool name")

    monkeypatch.setattr(graph_module, "chat", fake_chat)
    monkeypatch.setattr(graph_module, "call_tool", unexpected_call)

    state = await graph_module.run_turn([{"role": "user", "content": "do something bad"}], [make_tool()])

    assert state["tool_used"] == "delete_universe"
    assert state["connector_used"] == ""
    assert "not available" in state["messages"][-1]["content"]
