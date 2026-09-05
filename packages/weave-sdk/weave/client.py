"""The weave SDK — `import weave; client = weave.connect(...)`.

This is a registration/config client, not a local orchestration engine:
calling add_tool() tells Weave's hosted core about a service you already
run publicly; Weave's own infra (mcp-gateway, orchestrator) does the
actual reasoning and tool-calling later. Nothing here runs an MCP server,
holds conversation state, or calls an LLM — see docs/architecture/
ARCHITECTURE.md for why (core is the only tier with Weave's own DB
credentials; keeping execution server-side is what makes that true).

Self-contained by design: this package bundles its own generated gRPC
stubs (./gen, see _core_client.py) rather than depending on any other
package in this monorepo. The intent is that `weave` is installable and
usable inside *any* Python project — not just the reference projects that
happen to live alongside this repo (`tarang-electronics`,
`suvidha-finserve`) — with nothing beyond this one package (plus grpcio/
protobuf) on the importing project's side. Pre-publish, that means a
path or git dependency on `packages/weave-sdk` (see initialize.sh for
the codegen step any consumer needs run once); post-publish, `pip install
weave-sdk` with no further setup.
"""

import asyncio
import json
import threading
from dataclasses import dataclass
from typing import Any

# Import order matters: importing ._core_client first is what puts this
# package's bundled gen/ on sys.path (see that module's docstring), which
# is what makes the core.data_access.v1 import below resolve at all.
from ._core_client import CoreClient, bearer_metadata

from core.data_access.v1 import auth_pb2, bot_profile_pb2, connector_pb2, http_tool_pb2, tenant_pb2

from .openapi import tools_from_openapi

_DEFAULT_SCHEMA = {"type": "object", "properties": {}}


@dataclass
class RegisteredTool:
    id: str
    name: str
    description: str
    endpoint: str
    method: str
    visibility: str
    category: str
    auth_mode: str = "none"


@dataclass
class BotProfileHandle:
    id: str
    name: str
    visibility: str


class WeaveClient:
    """Async client. Prefer `weave.connect()` (sync) unless your own code
    is already async — see SyncWeaveClient below."""

    def __init__(self, core: CoreClient, tenant_id: str, token: str):
        self._core = core
        self._tenant_id = tenant_id
        self._token = token

    @property
    def tenant_id(self) -> str:
        return self._tenant_id

    @property
    def access_token(self) -> str:
        """The JWT this client authenticated with — the same shape and
        signing key core's ChatService.ChatStream RPC (protos/
        orchestrator/v1/chat.proto) expects as an `authorization: Bearer
        <token>` gRPC metadata entry. Exposed for a caller building its
        own channel (a real chat surface talking to orchestrator
        directly, not through this SDK — see _core_client.py's docstring
        for why this SDK itself never calls chat/ChatStream): it still
        needs *some* way to reuse the identity this client already
        established, rather than re-implementing sign_up/connect's
        Login call a second time just to get a token."""
        return self._token

    async def add_tool(
        self,
        *,
        name: str,
        description: str,
        endpoint: str,
        method: str = "GET",
        params_schema: dict[str, Any] | None = None,
        credential_secret: str | None = None,
        visibility: str = "internal",
        category: str = "general",
        auth_mode: str = "none",
    ) -> RegisteredTool:
        """Registers ONE public HTTP endpoint as a tool your bot can call.

        Call this once per endpoint you want Weave to reason over — not
        once per endpoint you have. If your API has 70 routes and only 40
        are relevant to a bot (the rest might be internal plumbing, admin-
        only, deprecated, or simply not something a customer or staff
        member would ever need to ask about), call add_tool() 40 times and
        leave the other 30 unregistered entirely — or, if you already have
        an OpenAPI spec for your API, see `add_tools_from_openapi()` below
        to do the same 40-of-70 selection in one call instead of 40.
        There's no "register everything" default either way: every tool a
        bot can reach should be a deliberate decision (this is also why
        `visibility` below has no safe default — see ARCHITECTURE.md §3),
        and an unregistered endpoint is simply invisible to Weave, not a
        partially-exposed one.

        endpoint may contain {param} placeholders matching keys in
        params_schema's properties — those are substituted into the URL
        path rather than sent as query/body params (e.g.
        "https://api.acme.com/orders/{order_id}/status").

        description is mandatory and load-bearing, not decoration: it's
        what the model uses to decide whether and how to call this tool,
        and it travels with the tool's result too (PLAN.md's tool-
        description requirement) — write it like documentation, not a label.

        visibility: "internal" (default, staff-only bot profiles) or
        "external" (also usable by customer-facing bot profiles) — see
        docs/architecture/ARCHITECTURE.md §3. category: "general"
        (default) or "analytics" — tags this tool for the analytics
        specialist route (docs/architecture/ARCHITECTURE.md §3's
        multi-agent supervisor).

        auth_mode: "none" (default — credential_secret, if any, is sent
        as a static "Authorization: Bearer" header, the same secret for
        every caller) or "user_token" — for an endpoint that must be
        scoped to the specific signed-in Weave user asking (e.g. a
        finance app's "my own transactions"), not just any authorized
        caller. Every ChatStream turn is already authenticated, so this
        restricts a tool to real, registered users, never opens one up.
        Requires credential_secret to be set — it's reinterpreted as an
        HMAC signing key rather than a bearer token: Weave verifies which
        user is asking, computes signature = HMAC-SHA256(credential_secret,
        f"{tenant_id}:{user_id}"), and sends it to your endpoint as
        X-Weave-User-Id / X-Weave-Tenant-Id / X-Weave-User-Signature
        headers — verify that signature with the same secret you set here
        (the same webhook-signing-secret pattern Stripe/GitHub use for
        verifying inbound requests) before trusting X-Weave-User-Id, then
        scope your response to that user however you map Weave users to
        your own user records.
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
                visibility=visibility,
                category=category,
                auth_mode=auth_mode,
            ),
            metadata=bearer_metadata(self._token),
        )
        t = resp.http_tool
        return RegisteredTool(
            id=t._id, name=t.name, description=t.description, endpoint=t.http_endpoint, method=t.http_method,
            visibility=t.visibility, category=t.category, auth_mode=t.auth_mode or "none",
        )

    async def add_tools_from_openapi(
        self,
        spec: dict[str, Any],
        *,
        base_url: str | None = None,
        include: set[str] | list[str] | None = None,
        exclude: set[str] | list[str] | None = None,
        default_visibility: str = "internal",
        default_category: str = "general",
        default_auth_mode: str = "none",
        credential_secret: str | None = None,
    ) -> list[RegisteredTool]:
        """Bulk-registers tools from a tenant's own OpenAPI 3.x document —
        the answer to "I have 70 endpoints and only want 40 of them as
        tools" without 40 individual add_tool() calls. See `weave.openapi`
        module docstring for the full design rationale (the gRPC-service
        equivalent: a descriptor-driven tool generator with an allow/deny
        list, same idea applied to a REST API's own descriptor — its
        OpenAPI spec — instead of a compiled proto descriptor).

        include/exclude are operationId sets, mutually exclusive: pass
        `include` to register only those 40, or `exclude` to register
        everything except a named 30. Passing neither registers every
        operation in the spec — rarely right for a real integrator with a
        large surface, so most callers should pass one.

        Every selected operation still needs a `description` or `summary`
        in the spec (same non-negotiable rule as `add_tool()`) and still
        gets an explicit visibility/category — `default_visibility`/
        `default_category` apply spec-wide, but any operation can override
        either via the `x-weave-visibility`/`x-weave-category` OpenAPI
        extension keys, the bulk-path equivalent of `add_tool()`'s
        `visibility=`/`category=` arguments. `default_auth_mode`/
        `x-weave-auth-mode` work the same way for `add_tool()`'s
        `auth_mode=` — e.g. a finance app's spec might mark every
        `/me/*` operation `x-weave-auth-mode: user_token` while leaving
        public endpoints at the "none" default. `credential_secret` here
        is one shared secret applied to every tool this call registers
        (bulk registration doesn't support a different secret per
        operation) — fine for a single signing key covering a whole
        `user_token`-tagged batch, call `add_tool()` directly instead if
        different operations genuinely need different secrets.
        """
        planned = tools_from_openapi(
            spec,
            base_url=base_url,
            include=include,
            exclude=exclude,
            default_visibility=default_visibility,
            default_category=default_category,
            default_auth_mode=default_auth_mode,
        )
        return [
            await self.add_tool(
                name=t.name,
                description=t.description,
                endpoint=t.endpoint,
                method=t.method,
                params_schema=t.params_schema,
                credential_secret=credential_secret,
                visibility=t.visibility,
                category=t.category,
                auth_mode=t.auth_mode,
            )
            for t in planned
        ]

    async def list_tools(self) -> list[RegisteredTool]:
        resp = await self._core.http_tool.ListHttpTools(http_tool_pb2.ListHttpToolsRequest(tenant_id=self._tenant_id))
        return [
            RegisteredTool(
                id=t._id, name=t.name, description=t.description, endpoint=t.http_endpoint, method=t.http_method,
                visibility=t.visibility, category=t.category, auth_mode=t.auth_mode or "none",
            )
            for t in resp.http_tools
        ]

    async def remove_tool(self, tool_id: str) -> None:
        await self._core.http_tool.DeregisterHttpTool(
            http_tool_pb2.DeregisterHttpToolRequest(tenant_id=self._tenant_id, http_tool_id=tool_id),
            metadata=bearer_metadata(self._token),
        )

    async def create_bot_profile(
        self,
        *,
        name: str,
        channels: list[str],
        roles_allowed: list[str],
        persona: str = "",
        connector_ids: list[str] | None = None,
        visibility: str = "internal",
        guardrails: list[str] | None = None,
        web_search_enabled: bool = False,
        llm_provider: str = "",
        llm_model: str = "",
    ) -> BotProfileHandle:
        """Creates a bot profile — the unit a business points a channel at.
        A tenant typically creates more than one (e.g. one per audience,
        like the "external"/"internal" pair every reference project in
        this repo's sibling demos uses): each gets its own persona,
        guardrails, and channel, but all draw from the same tool registry
        — the split is which tools/behavior a profile gets, not a second
        copy of your systems.

        roles_allowed is a list of role names ("owner"|"admin"|"staff"|
        "customer"). connector_ids defaults to this tenant's single
        weave_managed connector (every add_tool()'d tool lives there) if
        omitted, since that's the common case for a business using only
        the HTTP-tool SDK path rather than a hand-rolled MCP server.

        persona is this profile's entire system prompt, verbatim — not a
        file path (there's nothing on Weave's side that reads tenant
        files). Write the actual prompt text here: who this bot is, its
        tone, and its task scope, e.g. "You are Suvidha FinServe's client-
        facing assistant. Only discuss the caller's own company's
        invoices and filings; never reference another client by name."
        Left as "" (the default), the bot still works but gets a generic
        fallback prompt instead of anything specific to your business —
        write one per profile rather than leaving it blank.

        guardrails is this profile's own list of disclosure rules, e.g.
        "Never reveal another customer's contact details." — free text,
        one rule per list entry, enforced only when visibility=="external"
        (docs/architecture/SECURITY.md §6). Each bot profile has its own
        guardrails list; an "internal" and "external" profile on the same
        tenant commonly carry entirely different rules (or none, for the
        staff-facing one) rather than sharing one.

        llm_provider/llm_model choose which LLM backend generates this
        profile's turns — "" (default) or "ollama" for orchestrator's
        local model (OLLAMA_MODEL/OLLAMA_HOST), "openai" for anything
        speaking the OpenAI chat-completions wire format (OpenAI itself,
        Azure OpenAI, Groq, a self-hosted vLLM/LM Studio server — which
        one is determined by orchestrator's own OPENAI_BASE_URL
        configuration, not by a value you set here). llm_model is the
        model name/id to request from whichever provider that resolves
        to, e.g. "llama3.2:3b" or "gpt-4o-mini" — left "", each provider
        falls back to its own configured default. This is orchestrator
        picking between backends it already holds credentials for, not a
        place to supply your own API key (see the field's proto comment,
        protos/database/v1/bot_profile.proto, for why).
        """
        resolved_connector_ids = connector_ids
        if resolved_connector_ids is None:
            conn_resp = await self._core.connector.ListConnectors(
                connector_pb2.ListConnectorsRequest(tenant_id=self._tenant_id)
            )
            resolved_connector_ids = [c._id for c in conn_resp.connectors if c.name == "weave_managed"]

        role_enum = {"owner": 1, "admin": 2, "staff": 3, "customer": 4}
        resp = await self._core.bot_profile.CreateBotProfile(
            bot_profile_pb2.CreateBotProfileRequest(
                tenant_id=self._tenant_id,
                name=name,
                persona=persona,
                connector_ids=resolved_connector_ids,
                channels=channels,
                roles_allowed=[role_enum[r] for r in roles_allowed],
                visibility=visibility,
                guardrails=guardrails or [],
                web_search_enabled=web_search_enabled,
                llm_provider=llm_provider,
                llm_model=llm_model,
            ),
            metadata=bearer_metadata(self._token),
        )
        p = resp.bot_profile
        return BotProfileHandle(id=p._id, name=p.name, visibility=p.visibility)

    async def close(self) -> None:
        await self._core.close()

    async def __aenter__(self) -> "WeaveClient":
        return self

    async def __aexit__(self, *exc: object) -> None:
        await self.close()


async def sign_up(
    *, display_name: str, email: str, password: str, tenant_type: str = "business", core_addr: str = "localhost:9090"
) -> str:
    """Step 1 of onboarding a new business onto Weave: CreateTenant +
    Register(owner) — core's real public bootstrap RPCs, unauthenticated
    by design (there's no token to present before a tenant/user exists at
    all), not a special-cased dev shortcut. Returns the new tenant_id.

    Exists as its own function — rather than folding into connect_async,
    which only ever logs in — so an integrator's own setup script can
    narrate "sign up" as an explicit, separate step (see
    tarang-electronics/onboard.py or suvidha-finserve/onboard.py for a
    worked example) without reaching past this SDK for
    `weave_shared_clients`/raw core protos to do it — this package is
    meant to be everything an external project needs, on its own.
    """
    core = CoreClient(core_addr)
    try:
        tenant_resp = await core.tenant.CreateTenant(
            tenant_pb2.CreateTenantRequest(display_name=display_name, tenant_type=tenant_type)
        )
        tenant_id = tenant_resp.tenant._id
        await core.auth.Register(
            auth_pb2.RegisterRequest(tenant_id=tenant_id, email=email, password=password, role=1)  # 1 == owner
        )
        return tenant_id
    finally:
        await core.close()


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

    @property
    def tenant_id(self) -> str:
        return self._async_client.tenant_id

    @property
    def access_token(self) -> str:
        """See WeaveClient.access_token."""
        return self._async_client.access_token

    def add_tool(self, **kwargs: Any) -> RegisteredTool:
        return self._run(self._async_client.add_tool(**kwargs))

    def add_tools_from_openapi(self, spec: dict[str, Any], **kwargs: Any) -> list[RegisteredTool]:
        return self._run(self._async_client.add_tools_from_openapi(spec, **kwargs))

    def list_tools(self) -> list[RegisteredTool]:
        return self._run(self._async_client.list_tools())

    def remove_tool(self, tool_id: str) -> None:
        self._run(self._async_client.remove_tool(tool_id))

    def create_bot_profile(self, **kwargs: Any) -> BotProfileHandle:
        return self._run(self._async_client.create_bot_profile(**kwargs))

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
