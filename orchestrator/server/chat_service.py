"""orchestrator's chat gRPC server (docs/architecture/ARCHITECTURE.md §1).
Resolves tenant/role from the caller's JWT (never a client-supplied
tenant_id — docs/architecture/SECURITY.md §2), assembles that tenant's
tools dynamically for the requested channel, runs the planner/tool-call
decision, and streams the answer back token by token.
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

from llm.ollama_client import chat_stream  # noqa: E402
from server.auth import InvalidTokenError, bearer_token_from_metadata, verify_access_token  # noqa: E402
from server.graph import run_turn  # noqa: E402
from tools.assembly import ToolAssemblyError, assemble_tools  # noqa: E402

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("orchestrator.chat_service")

CORE_ADDR = os.environ.get("CORE_ADDR", "localhost:9090")
GRPC_ADDR = os.environ.get("GRPC_ADDR", "[::]:9091")

DEFAULT_SYSTEM_PROMPT = (
    "You are a helpful assistant for this business. Use the tools available to you "
    "when they help answer the user's request; otherwise answer directly and concisely."
)


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
            tools = await assemble_tools(
                self._core,
                tenant_id=claims.tenant_id,
                channel=request.channel,
                role=claims.role,
                token=token,
            )
        except ToolAssemblyError as exc:
            await context.abort(grpc.StatusCode.FAILED_PRECONDITION, str(exc))
            return

        messages = [
            {"role": "system", "content": DEFAULT_SYSTEM_PROMPT},
            {"role": "user", "content": request.message},
        ]

        logger.info(
            "tenant=%s channel=%s tools_available=%d message=%r",
            claims.tenant_id,
            request.channel,
            len(tools),
            request.message,
        )

        state = await run_turn(messages, tools)
        if state["tool_used"]:
            logger.info("tool_used=%s connector_used=%s", state["tool_used"], state["connector_used"])

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
