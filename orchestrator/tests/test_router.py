import server.router as router_module
from llm.ollama_client import ChatResult
from server.router import classify_route


async def test_no_registered_tools_and_no_web_search_always_routes_to_tools(monkeypatch):
    async def boom(*args, **kwargs):
        raise AssertionError("chat() should not be called when there's nothing else to route to")

    monkeypatch.setattr(router_module, "chat", boom)

    route = await classify_route("what's the weather like?", has_registered_tools=False)
    assert route == "tools"


async def test_only_tools_available_skips_llm_call(monkeypatch):
    async def boom(*args, **kwargs):
        raise AssertionError("chat() should not be called when tools is the only possible route")

    monkeypatch.setattr(router_module, "chat", boom)

    route = await classify_route(
        "what's the status of my order?", has_registered_tools=True, has_analytics_tools=False, web_search_enabled=False
    )
    assert route == "tools"


async def test_classifies_business_question_as_tools(monkeypatch):
    async def fake_chat(messages):
        return ChatResult(content="TOOLS", tool_calls=[])

    monkeypatch.setattr(router_module, "chat", fake_chat)

    route = await classify_route(
        "what's the status of my order 123?", has_registered_tools=True, web_search_enabled=True
    )
    assert route == "tools"


async def test_classifies_general_question_as_web_when_enabled(monkeypatch):
    async def fake_chat(messages):
        return ChatResult(content="WEB", tool_calls=[])

    monkeypatch.setattr(router_module, "chat", fake_chat)

    route = await classify_route(
        "what's the capital of France?", has_registered_tools=True, web_search_enabled=True
    )
    assert route == "web"


async def test_web_route_never_returned_when_search_disabled(monkeypatch):
    async def fake_chat(messages):
        # Even if the model somehow says WEB, WEB was never offered as a
        # valid category, so it must not surface as the chosen route.
        return ChatResult(content="WEB", tool_calls=[])

    monkeypatch.setattr(router_module, "chat", fake_chat)

    route = await classify_route(
        "what's the capital of France?",
        has_registered_tools=True,
        has_analytics_tools=True,
        web_search_enabled=False,
    )
    assert route in ("tools", "analytics")


async def test_classifies_analytics_question_as_analytics(monkeypatch):
    async def fake_chat(messages):
        return ChatResult(content="ANALYTICS", tool_calls=[])

    monkeypatch.setattr(router_module, "chat", fake_chat)

    route = await classify_route(
        "what was our revenue last quarter?", has_registered_tools=True, has_analytics_tools=True
    )
    assert route == "analytics"


async def test_unparseable_verdict_defaults_to_tools(monkeypatch):
    async def fake_chat(messages):
        return ChatResult(content="uh, not sure", tool_calls=[])

    monkeypatch.setattr(router_module, "chat", fake_chat)

    route = await classify_route("ambiguous message", has_registered_tools=True, web_search_enabled=True)
    assert route == "tools"


async def test_classification_failure_defaults_to_tools(monkeypatch):
    async def failing_chat(messages):
        raise RuntimeError("ollama is down")

    monkeypatch.setattr(router_module, "chat", failing_chat)

    route = await classify_route("some message", has_registered_tools=True, web_search_enabled=True)
    assert route == "tools"
