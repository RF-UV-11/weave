import server.guardrails as guardrails_module
from llm.ollama_client import ChatResult
from server.guardrails import screen


async def test_no_guardrails_always_passes():
    result = await screen("anything at all, even secrets", [])
    assert result.ok is True


async def test_empty_text_always_passes():
    result = await screen("   ", ["never disclose anything"])
    assert result.ok is True


async def test_ok_verdict_passes(monkeypatch):
    async def fake_chat(messages):
        return ChatResult(content="OK", tool_calls=[])

    monkeypatch.setattr(guardrails_module, "chat", fake_chat)

    result = await screen("The order shipped yesterday.", ["Never disclose supplier names."])
    assert result.ok is True


async def test_violation_verdict_fails_with_reason(monkeypatch):
    async def fake_chat(messages):
        return ChatResult(content="VIOLATION: mentions supplier Acme Corp", tool_calls=[])

    monkeypatch.setattr(guardrails_module, "chat", fake_chat)

    result = await screen("Your order was supplied by Acme Corp.", ["Never disclose supplier names."])
    assert result.ok is False
    assert "Acme Corp" in result.reason


async def test_unparseable_verdict_fails_closed(monkeypatch):
    async def fake_chat(messages):
        return ChatResult(content="I'm not sure, maybe?", tool_calls=[])

    monkeypatch.setattr(guardrails_module, "chat", fake_chat)

    result = await screen("some text", ["a rule"])
    assert result.ok is False
    assert "unparseable" in result.reason


async def test_judge_call_failure_fails_closed(monkeypatch):
    async def failing_chat(messages):
        raise RuntimeError("ollama is down")

    monkeypatch.setattr(guardrails_module, "chat", failing_chat)

    result = await screen("some text", ["a rule"])
    assert result.ok is False
    assert "failed" in result.reason


async def test_prompt_includes_all_rules_and_text(monkeypatch):
    captured = {}

    async def fake_chat(messages):
        captured["messages"] = messages
        return ChatResult(content="OK", tool_calls=[])

    monkeypatch.setattr(guardrails_module, "chat", fake_chat)

    await screen("the text to check", ["rule one", "rule two"])

    user_message = captured["messages"][1]["content"]
    assert "rule one" in user_message
    assert "rule two" in user_message
    assert "the text to check" in user_message
