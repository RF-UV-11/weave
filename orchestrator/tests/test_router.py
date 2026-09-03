import server.router as router_module
from llm.ollama_client import ChatResult
from server.router import classify_route


async def test_no_registered_tools_always_routes_to_tools(monkeypatch):
    async def boom(*args, **kwargs):
        raise AssertionError("chat() should not be called when there are no registered tools")

    monkeypatch.setattr(router_module, "chat", boom)

    route = await classify_route("what's the weather like?", has_registered_tools=False)
    assert route == "tools"


async def test_classifies_business_question_as_tools(monkeypatch):
    async def fake_chat(messages):
        return ChatResult(content="TOOLS", tool_calls=[])

    monkeypatch.setattr(router_module, "chat", fake_chat)

    route = await classify_route("what's the status of my order 123?", has_registered_tools=True)
    assert route == "tools"


async def test_classifies_general_question_as_web(monkeypatch):
    async def fake_chat(messages):
        return ChatResult(content="WEB", tool_calls=[])

    monkeypatch.setattr(router_module, "chat", fake_chat)

    route = await classify_route("what's the capital of France?", has_registered_tools=True)
    assert route == "web"


async def test_unparseable_verdict_defaults_to_tools(monkeypatch):
    async def fake_chat(messages):
        return ChatResult(content="uh, not sure", tool_calls=[])

    monkeypatch.setattr(router_module, "chat", fake_chat)

    route = await classify_route("ambiguous message", has_registered_tools=True)
    assert route == "tools"


async def test_classification_failure_defaults_to_tools(monkeypatch):
    async def failing_chat(messages):
        raise RuntimeError("ollama is down")

    monkeypatch.setattr(router_module, "chat", failing_chat)

    route = await classify_route("some message", has_registered_tools=True)
    assert route == "tools"
