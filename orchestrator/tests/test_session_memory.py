from types import SimpleNamespace
from unittest.mock import AsyncMock

from server.session_memory import persist_turn, resolve_session


def make_core(*, create_session=None, get_messages=None, append_message=None):
    core = SimpleNamespace()
    core.chat = SimpleNamespace(
        CreateSession=AsyncMock(return_value=create_session, side_effect=None),
        GetSessionMessages=AsyncMock(return_value=get_messages, side_effect=None),
        AppendMessage=AsyncMock(return_value=append_message, side_effect=None),
    )
    return core


async def test_resolve_session_creates_new_session_when_none_given():
    core = make_core(create_session=SimpleNamespace(session=SimpleNamespace(_id="ses_new")))

    session_id, prior = await resolve_session(
        core, tenant_id="tnt_1", user_id="usr_1", profile_id="profile_1", channel="web-widget", token="tok", session_id=""
    )

    assert session_id == "ses_new"
    assert prior == []
    core.chat.CreateSession.assert_awaited_once()
    core.chat.GetSessionMessages.assert_not_awaited()


async def test_resolve_session_loads_prior_replayable_messages():
    messages = [
        SimpleNamespace(role="user", content="hi"),
        SimpleNamespace(role="assistant", content="hello!"),
        SimpleNamespace(role="tool", content="raw tool result, should be dropped"),
    ]
    core = make_core(get_messages=SimpleNamespace(messages=messages))

    session_id, prior = await resolve_session(
        core, tenant_id="tnt_1", user_id="usr_1", profile_id="profile_1", channel="web-widget", token="tok",
        session_id="ses_existing",
    )

    assert session_id == "ses_existing"
    assert prior == [{"role": "user", "content": "hi"}, {"role": "assistant", "content": "hello!"}]
    core.chat.CreateSession.assert_not_awaited()


async def test_resolve_session_fails_soft_when_create_session_errors():
    core = make_core()
    core.chat.CreateSession.side_effect = RuntimeError("core is down")

    session_id, prior = await resolve_session(
        core, tenant_id="tnt_1", user_id="usr_1", profile_id="profile_1", channel="web-widget", token="tok", session_id=""
    )

    assert session_id == ""
    assert prior == []


async def test_resolve_session_fails_soft_when_session_id_is_stale():
    core = make_core()
    core.chat.GetSessionMessages.side_effect = RuntimeError("session not found")

    session_id, prior = await resolve_session(
        core, tenant_id="tnt_1", user_id="usr_1", profile_id="profile_1", channel="web-widget", token="tok",
        session_id="ses_stale",
    )

    # session_id is preserved (not discarded) even though history load
    # failed — persist_turn will still try to append under it.
    assert session_id == "ses_stale"
    assert prior == []


async def test_persist_turn_appends_both_sides():
    core = make_core()

    await persist_turn(
        core, tenant_id="tnt_1", session_id="ses_1", token="tok",
        user_message="what's my order status?", assistant_message="it shipped!",
        tool_used="track_order", connector_used="weave_managed",
    )

    assert core.chat.AppendMessage.await_count == 2
    first_call, second_call = core.chat.AppendMessage.call_args_list
    assert first_call.args[0].role == "user"
    assert first_call.args[0].content == "what's my order status?"
    assert second_call.args[0].role == "assistant"
    assert second_call.args[0].content == "it shipped!"
    assert second_call.args[0].tool_used == "track_order"
    assert second_call.args[0].connector_used == "weave_managed"


async def test_persist_turn_is_noop_when_session_id_empty():
    core = make_core()

    await persist_turn(
        core, tenant_id="tnt_1", session_id="", token="tok",
        user_message="hi", assistant_message="hello", tool_used="", connector_used="",
    )

    core.chat.AppendMessage.assert_not_awaited()


async def test_persist_turn_fails_soft_on_error():
    core = make_core()
    core.chat.AppendMessage.side_effect = RuntimeError("core is down")

    # Should not raise.
    await persist_turn(
        core, tenant_id="tnt_1", session_id="ses_1", token="tok",
        user_message="hi", assistant_message="hello", tool_used="", connector_used="",
    )
