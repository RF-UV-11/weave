"""Long-term/semantic memory (docs/architecture/ARCHITECTURE.md §5):
cross-session recall, unlike server/session_memory.py's short-term tier
which only ever sees turns within one session_id. orchestrator computes
the embedding (it holds LLM/embedding-model access — core never runs
inference) via Ollama and calls core's MemoryService, which is the only
tier holding the actual Qdrant connection.

Deliberate scope simplification: every user turn's raw message is stored
as one memory point, verbatim — there's no LLM-driven fact extraction
("the user's name is Jordan" distilled from a longer message) or
deduplication. Semantic search ranks by similarity regardless, so a
repeated fact just reinforces rather than corrupting recall; a real
fact-extraction pass is a natural follow-up, not required to make cross-
session recall genuinely work (verified live — see PLAN.md's Phase 3.8
notes). Fails soft, same posture as session_memory.py: a core/Qdrant
outage degrades a turn to "no long-term recall," never fails it.
"""

import logging

from weave_shared_clients import bearer_metadata

from core.data_access.v1 import memory_pb2
from llm.ollama_client import embed

logger = logging.getLogger("orchestrator.semantic_memory")

DEFAULT_TOP_K = 5


async def recall(core, *, tenant_id: str, user_id: str, token: str, query_text: str, top_k: int = DEFAULT_TOP_K) -> list[str]:
    """Returns up to top_k semantically relevant facts from this user's
    long-term memory, most relevant first. Empty (not an error) if
    nothing relevant exists yet or embedding/search fails."""
    try:
        query_embedding = await embed(query_text)
        resp = await core.memory.SearchMemory(
            memory_pb2.SearchMemoryRequest(
                tenant_id=tenant_id, user_id=user_id, query_embedding=query_embedding, top_k=top_k
            ),
            metadata=bearer_metadata(token),
        )
        return [r.text for r in resp.results]
    except Exception:  # noqa: BLE001 - fail soft, see module docstring
        logger.warning("semantic_memory: recall failed for tenant=%s user=%s", tenant_id, user_id, exc_info=True)
        return []


async def remember(core, *, tenant_id: str, user_id: str, token: str, text: str) -> None:
    """Stores one fact (the raw user message, see module docstring) for
    future cross-session recall. A no-op on any failure, never raises."""
    if not text.strip():
        return
    try:
        embedding = await embed(text)
        await core.memory.UpsertMemory(
            memory_pb2.UpsertMemoryRequest(tenant_id=tenant_id, user_id=user_id, text=text, embedding=embedding),
            metadata=bearer_metadata(token),
        )
    except Exception:  # noqa: BLE001 - fail soft, see module docstring
        logger.warning("semantic_memory: remember failed for tenant=%s user=%s", tenant_id, user_id, exc_info=True)
