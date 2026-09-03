"""orchestrator's chat gRPC server (docs/architecture/ARCHITECTURE.md §1).
Resolves tenant/role from the caller's JWT (never a client-supplied
tenant_id — docs/architecture/SECURITY.md §2), routes the turn to a
specialist agent (tools vs web — see server/router.py), runs the
planner/tool-call decision, and streams the answer back.

Streaming is real (token-by-token, as the model generates it) for
profiles with no guardrails. For a guardrail-protected external profile
(docs/architecture/SECURITY.md §6), the full answer is generated first,
screened, and only then sent — a hard guardrail can't coexist with
real-time streaming, since a token already sent over the wire can't be
unsent. See server/guardrails.py for the full reasoning.
"""

import asyncio
import logging
import os
import sys
from pathlib import Path

_ORCH_DIR = Path(__file__).parent.parent
sys.path.insert(0, str(_ORCH_DIR))
# chat_pb2_grpc.py imports `from orchestrator.v1 import chat_pb2` (protoc's
# default non-package-relative import, relative to the -I proto root) —
# gen/ itself has to be importable as a top-level "orchestrator" package,
# same reason weave_shared_clients puts its own gen/ on sys.path.
sys.path.insert(0, str(_ORCH_DIR / "gen"))

import grpc  # noqa: E402

from gen.orchestrator.v1 import chat_pb2, chat_pb2_grpc  # noqa: E402
from weave_shared_clients import CoreClient  # noqa: E402

from llm.ollama_client import chat, chat_stream  # noqa: E402
from server.auth import InvalidTokenError, bearer_token_from_metadata, verify_access_token  # noqa: E402
from server.graph import run_turn  # noqa: E402
from server.guardrails import screen  # noqa: E402
from server.router import classify_route  # noqa: E402
from server.web_search import WEB_SEARCH_TOOL  # noqa: E402
from tools.assembly import ToolAssemblyError, assemble_tools  # noqa: E402

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("orchestrator.chat_service")

CORE_ADDR = os.environ.get("CORE_ADDR", "localhost:9090")
GRPC_ADDR = os.environ.get("GRPC_ADDR", "[::]:9091")

DEFAULT_SYSTEM_PROMPT = (
    "You are a helpful assistant for this business. Use the tools available to you "
    "when they help answer the user's request; otherwise answer directly and concisely."
)

GUARDRAIL_REFUSAL = "I'm not able to share that information."

# Simulated streaming for the guardrail-buffered path — the answer is
# already fully generated and approved by the time this runs, so this is
# purely a UX choice (chunked delivery, not a safety mechanism).
_CHUNK_WORDS = 3


def _chunk_text(text: str) -> list[str]:
    words = text.split(" ")
    chunks = []
    for i in range(0, len(words), _CHUNK_WORDS):
        piece = " ".join(words[i : i + _CHUNK_WORDS])
        chunks.append(piece + (" " if i + _CHUNK_WORDS < len(words) else ""))
    return chunks


class ChatServiceServicer(chat_pb2_grpc.ChatServiceServicer):
    def __init__(self, core: CoreClient):
        self._core = core

    async def ChatStream(self, request: chat_pb2.ChatStreamRequest, context: grpc.aio.ServicerContext):
        try:
            token = bearer_token_from_metadata(context.invocation_metadata())
            claims = verify_access_token(token)
        except InvalidTokenError as exc:
            await context.abort(grpc.StatusCode.UNAUTHENTICATED, str(exc))
            return

        try:
            assembly = await assemble_tools(
                self._core,
                tenant_id=claims.tenant_id,
                channel=request.channel,
                role=claims.role,
                token=token,
            )
        except ToolAssemblyError as exc:
            await context.abort(grpc.StatusCode.FAILED_PRECONDITION, str(exc))
            return

        route = await classify_route(request.message, has_registered_tools=bool(assembly.tools))
        chosen_tools = [WEB_SEARCH_TOOL] if route == "web" else assembly.tools

        messages = [
            {"role": "system", "content": DEFAULT_SYSTEM_PROMPT},
            {"role": "user", "content": request.message},
        ]

        logger.info(
            "tenant=%s channel=%s profile=%s visibility=%s route=%s tools_available=%d message=%r",
            claims.tenant_id,
            request.channel,
            assembly.profile_name,
            assembly.visibility,
            route,
            len(chosen_tools),
            request.message,
        )

        state = await run_turn(
            messages,
            chosen_tools,
            guardrails=assembly.guardrails if assembly.guardrails_active else None,
        )
        if state["tool_used"]:
            logger.info("tool_used=%s connector_used=%s", state["tool_used"], state["connector_used"])

        if assembly.guardrails_active:
            async for resp in self._guarded_response(state, assembly.guardrails, request.session_id):
                yield resp
            return

        async for token_text in chat_stream(state["messages"]):
            yield chat_pb2.ChatStreamResponse(
                token=token_text,
                done=False,
                session_id=request.session_id,
                tool_used=state["tool_used"],
                connector_used=state["connector_used"],
            )

        yield chat_pb2.ChatStreamResponse(
            token="",
            done=True,
            session_id=request.session_id,
            tool_used=state["tool_used"],
            connector_used=state["connector_used"],
        )

    async def _guarded_response(self, state, guardrails: list[str], session_id: str):
        result = await chat(state["messages"])
        verdict = await screen(result.content, guardrails)

        if not verdict.ok:
            logger.info("guardrail violation on final answer: %s", verdict.reason)
            yield chat_pb2.ChatStreamResponse(
                token=GUARDRAIL_REFUSAL,
                done=False,
                session_id=session_id,
                tool_used=state["tool_used"],
                connector_used=state["connector_used"],
            )
        else:
            for chunk in _chunk_text(result.content):
                yield chat_pb2.ChatStreamResponse(
                    token=chunk,
                    done=False,
                    session_id=session_id,
                    tool_used=state["tool_used"],
                    connector_used=state["connector_used"],
                )

        yield chat_pb2.ChatStreamResponse(
            token="",
            done=True,
            session_id=session_id,
            tool_used=state["tool_used"],
            connector_used=state["connector_used"],
        )


async def serve() -> None:
    core = CoreClient(CORE_ADDR)
    server = grpc.aio.server()
    chat_pb2_grpc.add_ChatServiceServicer_to_server(ChatServiceServicer(core), server)
    server.add_insecure_port(GRPC_ADDR)
    await server.start()
    logger.info("orchestrator listening on %s (core at %s)", GRPC_ADDR, CORE_ADDR)
    try:
        await server.wait_for_termination()
    finally:
        await core.close()


if __name__ == "__main__":
    asyncio.run(serve())
