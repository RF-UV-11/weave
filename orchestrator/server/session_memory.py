"""Short-term/session memory (docs/architecture/ARCHITECTURE.md §5):
orchestrator never holds conversation state itself — every read/write
goes through core's ChatService, same trust-boundary rule as everything
else on this tier (SECURITY.md §1's "orchestrator never holds a database
connection").

Deliberately fails soft, not closed: a core outage or a stale/foreign
session_id degrades a turn to "no prior context" rather than failing the
whole chat request. Memory is a UX enhancement to this turn, not a
precondition for answering it — the same "fails open on outage" posture
docs/architecture/SECURITY.md §5 takes for rate limiting, for the same
reason (the alternative — refusing to chat because memory is down — is
worse than a temporarily stateless turn).
"""

import logging

from weave_shared_clients import bearer_metadata

from core.data_access.v1 import chat_pb2

logger = logging.getLogger("orchestrator.session_memory")

# Only user/assistant turns are replayed into a future turn's context —
# a "tool" message is the raw result from a specific tool call scoped to
# the turn that made it (and already folded into that turn's assistant
# answer), not something that should be re-injected verbatim into later
# prompts.
_REPLAYABLE_ROLES = {"user", "assistant"}


async def resolve_session(
    core,
    *,
    tenant_id: str,
    user_id: str,
    profile_id: str,
    channel: str,
    token: str,
    session_id: str,
) -> tuple[str, list[dict[str, str]]]:
    """Returns (session_id, prior_messages). Creates a new session if
    session_id is empty; otherwise loads prior history for the given
    session_id, falling back to "new session, no history" if that
    session_id doesn't resolve for this tenant (stale reference, or core
    is unreachable) rather than failing the turn."""
    metadata = bearer_metadata(token)

    if not session_id:
        try:
            resp = await core.chat.CreateSession(
                chat_pb2.CreateSessionRequest(
                    tenant_id=tenant_id, user_id=user_id, bot_profile_id=profile_id, channel=channel
                ),
                metadata=metadata,
            )
            return resp.session._id, []
        except Exception:  # noqa: BLE001 - fail soft, see module docstring
            logger.warning("session_memory: CreateSession failed, continuing without persistence", exc_info=True)
            return "", []

    try:
        resp = await core.chat.GetSessionMessages(
            chat_pb2.GetSessionMessagesRequest(tenant_id=tenant_id, session_id=session_id),
            metadata=metadata,
        )
        prior = [{"role": m.role, "content": m.content} for m in resp.messages if m.role in _REPLAYABLE_ROLES]
        return session_id, prior
    except Exception:  # noqa: BLE001 - fail soft, see module docstring
        logger.warning("session_memory: GetSessionMessages failed for session_id=%s, continuing without history", session_id, exc_info=True)
        return session_id, []


async def persist_turn(
    core,
    *,
    tenant_id: str,
    session_id: str,
    token: str,
    user_message: str,
    assistant_message: str,
    tool_used: str,
    connector_used: str,
) -> None:
    """Appends both sides of a turn. A no-op if session_id is empty
    (resolve_session already failed soft for this turn)."""
    if not session_id:
        return
    metadata = bearer_metadata(token)
    try:
        await core.chat.AppendMessage(
            chat_pb2.AppendMessageRequest(tenant_id=tenant_id, session_id=session_id, role="user", content=user_message),
            metadata=metadata,
        )
        await core.chat.AppendMessage(
            chat_pb2.AppendMessageRequest(
                tenant_id=tenant_id,
                session_id=session_id,
                role="assistant",
                content=assistant_message,
                tool_used=tool_used,
                connector_used=connector_used,
            ),
            metadata=metadata,
        )
    except Exception:  # noqa: BLE001 - fail soft, see module docstring
        logger.warning("session_memory: persist_turn failed for session_id=%s", session_id, exc_info=True)
