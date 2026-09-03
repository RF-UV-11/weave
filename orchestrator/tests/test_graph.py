import pytest

import server.graph as graph_module
from llm.ollama_client import ChatResult, ToolCall
from mcp_client.client import McpTool
from server.guardrails import ScreenResult
from server.web_search import WEB_SEARCH_TOOL
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


async def test_web_search_tool_dispatches_to_run_web_search_not_call_tool(monkeypatch):
    async def fake_chat(messages, tools=None):
        return ChatResult(content="", tool_calls=[ToolCall(name="web_search", arguments={"query": "weather in london"})])

    async def fake_run_web_search(query):
        assert query == "weather in london"
        return "It's raining, as usual."

    async def unexpected_call_tool(*args, **kwargs):
        raise AssertionError("call_tool (MCP path) should not be used for the built-in web_search tool")

    monkeypatch.setattr(graph_module, "chat", fake_chat)
    monkeypatch.setattr(graph_module, "run_web_search", fake_run_web_search)
    monkeypatch.setattr(graph_module, "call_tool", unexpected_call_tool)

    state = await graph_module.run_turn([{"role": "user", "content": "what's the weather?"}], [WEB_SEARCH_TOOL])

    assert state["tool_used"] == "web_search"
    assert "raining" in state["messages"][-1]["content"]


async def test_guardrail_violation_redacts_tool_result_before_context(monkeypatch):
    async def fake_chat(messages, tools=None):
        return ChatResult(content="", tool_calls=[ToolCall(name="book_appointment", arguments={})])

    async def fake_call_tool(endpoint, name, arguments):
        return "Sensitive: supplied by Acme Corp"

    async def fake_screen(text, guardrails):
        assert "Acme Corp" in text
        return ScreenResult(ok=False, reason="mentions supplier")

    monkeypatch.setattr(graph_module, "chat", fake_chat)
    monkeypatch.setattr(graph_module, "call_tool", fake_call_tool)
    monkeypatch.setattr(graph_module, "screen", fake_screen)

    state = await graph_module.run_turn(
        [{"role": "user", "content": "who supplies this?"}],
        [make_tool()],
        guardrails=["Never disclose supplier names."],
    )

    tool_message = state["messages"][-1]["content"]
    assert "Acme Corp" not in tool_message
    assert "withheld" in tool_message


async def test_guardrail_pass_leaves_tool_result_untouched(monkeypatch):
    async def fake_chat(messages, tools=None):
        return ChatResult(content="", tool_calls=[ToolCall(name="book_appointment", arguments={})])

    async def fake_call_tool(endpoint, name, arguments):
        return "Booked bkg_1"

    async def fake_screen(text, guardrails):
        return ScreenResult(ok=True)

    monkeypatch.setattr(graph_module, "chat", fake_chat)
    monkeypatch.setattr(graph_module, "call_tool", fake_call_tool)
    monkeypatch.setattr(graph_module, "screen", fake_screen)

    state = await graph_module.run_turn(
        [{"role": "user", "content": "book me a slot"}],
        [make_tool()],
        guardrails=["Never disclose supplier names."],
    )

    assert "Booked bkg_1" in state["messages"][-1]["content"]


async def test_no_guardrails_skips_screening_entirely(monkeypatch):
    async def fake_chat(messages, tools=None):
        return ChatResult(content="", tool_calls=[ToolCall(name="book_appointment", arguments={})])

    async def fake_call_tool(endpoint, name, arguments):
        return "Booked bkg_1"

    async def unexpected_screen(*args, **kwargs):
        raise AssertionError("screen() should not be called when no guardrails are configured")

    monkeypatch.setattr(graph_module, "chat", fake_chat)
    monkeypatch.setattr(graph_module, "call_tool", fake_call_tool)
    monkeypatch.setattr(graph_module, "screen", unexpected_screen)

    state = await graph_module.run_turn([{"role": "user", "content": "book me a slot"}], [make_tool()])

    assert "Booked bkg_1" in state["messages"][-1]["content"]
