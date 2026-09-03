from types import SimpleNamespace
from unittest.mock import AsyncMock

import pytest

import server.semantic_memory as semantic_memory_module
from server.semantic_memory import recall, remember


def make_core(*, search_response=None, upsert_response=None):
    core = SimpleNamespace()
    core.memory = SimpleNamespace(
        SearchMemory=AsyncMock(return_value=search_response),
        UpsertMemory=AsyncMock(return_value=upsert_response),
    )
    return core


async def test_recall_embeds_query_and_returns_texts(monkeypatch):
    fake_embed = AsyncMock(return_value=[0.1, 0.2, 0.3])
    monkeypatch.setattr(semantic_memory_module, "embed", fake_embed)
    core = make_core(
        search_response=SimpleNamespace(
            results=[SimpleNamespace(text="user's name is Jordan", memory_id="m1", score=0.9)]
        )
    )

    facts = await recall(core, tenant_id="tnt_1", user_id="usr_1", token="tok", query_text="what's my name?")

    assert facts == ["user's name is Jordan"]
    fake_embed.assert_awaited_once_with("what's my name?")
    core.memory.SearchMemory.assert_awaited_once()


async def test_recall_fails_soft_on_embed_error(monkeypatch):
    fake_embed = AsyncMock(side_effect=RuntimeError("ollama is down"))
    monkeypatch.setattr(semantic_memory_module, "embed", fake_embed)
    core = make_core()

    facts = await recall(core, tenant_id="tnt_1", user_id="usr_1", token="tok", query_text="anything")

    assert facts == []


async def test_recall_fails_soft_on_search_error(monkeypatch):
    fake_embed = AsyncMock(return_value=[0.1, 0.2])
    monkeypatch.setattr(semantic_memory_module, "embed", fake_embed)
    core = make_core()
    core.memory.SearchMemory.side_effect = RuntimeError("core is down")

    facts = await recall(core, tenant_id="tnt_1", user_id="usr_1", token="tok", query_text="anything")

    assert facts == []


async def test_remember_embeds_and_upserts(monkeypatch):
    fake_embed = AsyncMock(return_value=[0.1, 0.2, 0.3])
    monkeypatch.setattr(semantic_memory_module, "embed", fake_embed)
    core = make_core()

    await remember(core, tenant_id="tnt_1", user_id="usr_1", token="tok", text="my name is Jordan")

    fake_embed.assert_awaited_once_with("my name is Jordan")
    core.memory.UpsertMemory.assert_awaited_once()
    req = core.memory.UpsertMemory.call_args.args[0]
    assert req.text == "my name is Jordan"
    # proto's `repeated float` is float32, so the round-trip loses some
    # precision vs. the float64 literals above — approx, not exact equality.
    assert list(req.embedding) == pytest.approx([0.1, 0.2, 0.3], rel=1e-6)


async def test_remember_is_noop_for_blank_text(monkeypatch):
    fake_embed = AsyncMock()
    monkeypatch.setattr(semantic_memory_module, "embed", fake_embed)
    core = make_core()

    await remember(core, tenant_id="tnt_1", user_id="usr_1", token="tok", text="   ")

    fake_embed.assert_not_awaited()
    core.memory.UpsertMemory.assert_not_awaited()


async def test_remember_fails_soft_on_error(monkeypatch):
    fake_embed = AsyncMock(return_value=[0.1])
    monkeypatch.setattr(semantic_memory_module, "embed", fake_embed)
    core = make_core()
    core.memory.UpsertMemory.side_effect = RuntimeError("core is down")

    # Should not raise.
    await remember(core, tenant_id="tnt_1", user_id="usr_1", token="tok", text="anything")
