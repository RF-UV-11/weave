"""mcp-gateway's ASGI entrypoint: routes `/{tenant_id}/mcp` to a
freshly-built per-tenant MCP server (gateway.tenant_server), forwarding
the raw ASGI request. One process serves every tenant — tenant isolation
is enforced purely by path parsing + core's own tenant-scoped
ListHttpTools/RevealHttpToolCredential, the same isolation rule as every
other lookup in core (docs/architecture/SECURITY.md §2).
"""

import asyncio
import logging
import os
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))

from weave_shared_clients import CoreClient  # noqa: E402

from gateway import build_tenant_server  # noqa: E402

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("mcp_gateway")

CORE_ADDR = os.environ.get("CORE_ADDR", "localhost:9090")
GATEWAY_PORT = int(os.environ.get("GATEWAY_PORT", "8766"))

# Deliberately NOT constructed at import time: CoreClient wraps a
# grpc.aio channel, which is bound to whichever event loop is running
# when it's created. Module import happens before uvicorn.run() creates
# its own loop, so a module-level `core = CoreClient(...)` here would be
# a channel tied to the wrong loop — every RPC on it fails with "attached
# to a different loop" (the same class of bug already caught and fixed in
# packages/weave-sdk's sync client). Built instead in the real ASGI
# lifespan startup below, which runs inside uvicorn's actual loop.
core: CoreClient | None = None


async def _send_plain(send, status: int, text: str) -> None:
    await send({"type": "http.response.start", "status": status, "headers": [(b"content-type", b"text/plain")]})
    await send({"type": "http.response.body", "body": text.encode()})


async def _start_lifespan(sub_app) -> asyncio.Task:
    """Drives the startup half of the ASGI lifespan protocol for a
    freshly built per-tenant app.

    mcp's StreamableHTTPSessionManager needs its background task group
    initialized before it will handle any request ("Task group is not
    initialized. Make sure to use run()." otherwise) — that only happens
    in response to a real "lifespan.startup" event, which a bare
    "forward this one HTTP request" dispatcher never sends. This drives
    just enough of the lifespan protocol to satisfy that, without
    needing a full app-per-tenant server lifecycle.
    """
    startup_complete = asyncio.Event()
    startup_failed_msg: str | None = None
    sent_startup = False

    async def receive():
        nonlocal sent_startup
        if not sent_startup:
            sent_startup = True
            return {"type": "lifespan.startup"}
        # Park here — this task is cancelled once the request is done;
        # we never send a real "lifespan.shutdown" since there's nothing
        # tenant-specific left to clean up (the underlying core channel
        # is shared and outlives every individual request).
        await asyncio.Event().wait()

    async def send(message):
        nonlocal startup_failed_msg
        if message["type"] == "lifespan.startup.complete":
            startup_complete.set()
        elif message["type"] == "lifespan.startup.failed":
            startup_failed_msg = message.get("message", "unknown error")
            startup_complete.set()

    lifespan_task = asyncio.create_task(sub_app({"type": "lifespan"}, receive, send))
    await startup_complete.wait()
    if startup_failed_msg:
        lifespan_task.cancel()
        raise RuntimeError(f"mcp sub-app startup failed: {startup_failed_msg}")
    return lifespan_task


async def app(scope, receive, send):
    global core

    if scope["type"] == "lifespan":
        while True:
            message = await receive()
            if message["type"] == "lifespan.startup":
                core = CoreClient(CORE_ADDR)
                await send({"type": "lifespan.startup.complete"})
            elif message["type"] == "lifespan.shutdown":
                if core is not None:
                    await core.close()
                await send({"type": "lifespan.shutdown.complete"})
                return
        return

    if scope["type"] != "http":
        return
    assert core is not None, "received an HTTP request before lifespan startup ran"

    path = scope["path"]
    parts = [p for p in path.split("/") if p]
    if not parts:
        await _send_plain(send, 404, "expected /{tenant_id}/mcp")
        return

    tenant_id, rest = parts[0], parts[1:]
    sub_path = "/" + "/".join(rest) if rest else "/"

    tenant_server = build_tenant_server(core, tenant_id)
    sub_app = tenant_server.streamable_http_app(stateless_http=True)

    forwarded_scope = dict(scope)
    forwarded_scope["path"] = sub_path
    forwarded_scope["raw_path"] = sub_path.encode()

    lifespan_task = await _start_lifespan(sub_app)
    try:
        await sub_app(forwarded_scope, receive, send)
    finally:
        lifespan_task.cancel()


if __name__ == "__main__":
    import uvicorn

    logger.info("mcp-gateway listening on :%d (core at %s)", GATEWAY_PORT, CORE_ADDR)
    uvicorn.run(app, host="0.0.0.0", port=GATEWAY_PORT)
