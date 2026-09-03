"""The weave SDK — `import weave; client = weave.connect(...)`.

This is a registration/config client, not a local orchestration engine:
calling add_tool() tells Weave's hosted core about a service you already
run publicly; Weave's own infra (mcp-gateway, orchestrator) does the
actual reasoning and tool-calling later. Nothing here runs an MCP server,
holds conversation state, or calls an LLM — see docs/architecture/
ARCHITECTURE.md for why (core is the only tier with Weave's own DB
credentials; keeping execution server-side is what makes that true).
"""

import asyncio
import json
import threading
from dataclasses import dataclass
from typing import Any

from weave_shared_clients import CoreClient, bearer_metadata

from core.data_access.v1 import auth_pb2, http_tool_pb2

_DEFAULT_SCHEMA = {"type": "object", "properties": {}}


@dataclass
class RegisteredTool:
    id: str
    name: str
    description: str
    endpoint: str
    method: str


class WeaveClient:
    """Async client. Prefer `weave.connect()` (sync) unless your own code
    is already async — see SyncWeaveClient below."""

    def __init__(self, core: CoreClient, tenant_id: str, token: str):
        self._core = core
        self._tenant_id = tenant_id
        self._token = token

    async def add_tool(
        self,
        *,
        name: str,
        description: str,
        endpoint: str,
        method: str = "GET",
        params_schema: dict[str, Any] | None = None,
        credential_secret: str | None = None,
    ) -> RegisteredTool:
        """Registers a public HTTP endpoint as a tool your bot can call.

        endpoint may contain {param} placeholders matching keys in
        params_schema's properties — those are substituted into the URL
        path rather than sent as query/body params (e.g.
        "https://api.acme.com/orders/{order_id}/status").

        description is mandatory and load-bearing, not decoration: it's
        what the model uses to decide whether and how to call this tool,
        and it travels with the tool's result too (PLAN.md's tool-
        description requirement) — write it like documentation, not a label.
        """
        resp = await self._core.http_tool.RegisterHttpTool(
            http_tool_pb2.RegisterHttpToolRequest(
                tenant_id=self._tenant_id,
                name=name,
                description=description,
                http_endpoint=endpoint,
                http_method=method,
                params_schema=json.dumps(params_schema or _DEFAULT_SCHEMA),
                credential_secret=credential_secret or "",
            ),
            metadata=bearer_metadata(self._token),
        )
        t = resp.http_tool
        return RegisteredTool(id=t._id, name=t.name, description=t.description, endpoint=t.http_endpoint, method=t.http_method)

    async def list_tools(self) -> list[RegisteredTool]:
        resp = await self._core.http_tool.ListHttpTools(http_tool_pb2.ListHttpToolsRequest(tenant_id=self._tenant_id))
        return [
            RegisteredTool(id=t._id, name=t.name, description=t.description, endpoint=t.http_endpoint, method=t.http_method)
            for t in resp.http_tools
        ]

    async def remove_tool(self, tool_id: str) -> None:
        await self._core.http_tool.DeregisterHttpTool(
            http_tool_pb2.DeregisterHttpToolRequest(tenant_id=self._tenant_id, http_tool_id=tool_id),
            metadata=bearer_metadata(self._token),
        )

    async def close(self) -> None:
        await self._core.close()

    async def __aenter__(self) -> "WeaveClient":
        return self

    async def __aexit__(self, *exc: object) -> None:
        await self.close()


async def connect_async(*, tenant_id: str, email: str, password: str, core_addr: str = "localhost:9090") -> WeaveClient:
    core = CoreClient(core_addr)
    try:
        resp = await core.auth.Login(auth_pb2.LoginRequest(tenant_id=tenant_id, email=email, password=password))
    except Exception:
        await core.close()
        raise
    return WeaveClient(core, tenant_id, resp.access_token)


class SyncWeaveClient:
    """A synchronous facade over WeaveClient for non-async business
    codebases. Owns a single background thread running one persistent
    event loop for the client's lifetime — NOT `asyncio.run()` per call:
    the underlying gRPC channel is bound to the loop that created it, so
    a fresh loop per call would break on the second call (grpc.aio
    objects aren't portable across event loops). Every method call is
    submitted to that one loop via run_coroutine_threadsafe.
    """

    def __init__(self, *, tenant_id: str, email: str, password: str, core_addr: str = "localhost:9090"):
        self._loop = asyncio.new_event_loop()
        self._thread = threading.Thread(target=self._loop.run_forever, daemon=True)
        self._thread.start()
        self._async_client = self._run(
            connect_async(tenant_id=tenant_id, email=email, password=password, core_addr=core_addr)
        )

    def _run(self, coro):
        return asyncio.run_coroutine_threadsafe(coro, self._loop).result()

    def add_tool(self, **kwargs: Any) -> RegisteredTool:
        return self._run(self._async_client.add_tool(**kwargs))

    def list_tools(self) -> list[RegisteredTool]:
        return self._run(self._async_client.list_tools())

    def remove_tool(self, tool_id: str) -> None:
        self._run(self._async_client.remove_tool(tool_id))

    def close(self) -> None:
        self._run(self._async_client.close())
        self._loop.call_soon_threadsafe(self._loop.stop)
        self._thread.join(timeout=5)

    def __enter__(self) -> "SyncWeaveClient":
        return self

    def __exit__(self, *exc: object) -> None:
        self.close()


def connect(*, tenant_id: str, email: str, password: str, core_addr: str = "localhost:9090") -> SyncWeaveClient:
    """`client = weave.connect(tenant_id=..., email=..., password=...)`
    — the SDK's main entry point. Returns a synchronous client; use
    connect_async() directly if your own code is already async."""
    return SyncWeaveClient(tenant_id=tenant_id, email=email, password=password, core_addr=core_addr)
