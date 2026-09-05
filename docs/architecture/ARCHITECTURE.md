# Weave — System Architecture

Companion to `OVERVIEW.md` (the what/why) and `SECURITY.md` (the trust model). This doc is the how: request lifecycle, data model, and the mechanics of dynamic tool assembly.

---

## 1. Services

| Service | Language | Owns | Never does |
|---|---|---|---|
| `core` | Go | Tenant registry, connector registry, credential vault, chat/session/memory store, auth, billing/usage | Never runs LLM inference, never calls a tenant's MCP server directly |
| `orchestrator` | Python | Chat gRPC server (streaming), LangGraph planner + specialist agents, MCP client, dynamic tool assembly, RAG pipeline, memory assembly | Never holds a database connection — every read/write goes through `core` |
| `compute` | Python | Platform-generic compute modules (RAG reranking, faithfulness scoring) reached via routes on one server | Never hosts tenant-specific business logic — that belongs in a tenant's own connector |
| `connectors` (repo folder) | mixed | Reference/example MCP servers, connector scaffolding templates | Not deployed as part of the platform itself — these are examples and starter kits for tenants |
| *(external)* demo tenant reference projects | mixed | Fully worked examples of integrating the `weave` SDK, one per demo tenant | Never live in this repo, even under `connectors/` — a tenant's integration code belongs in the tenant's own codebase, same rule as any other tenant-owned MCP server. Currently: `tarang-electronics` (B2C retail) and `suvidha-finserve` (B2B professional services), sibling directories to `weave/`, each its own git repo. |
| `channels` | mixed | Thin per-channel translation to/from the chat API | Never contains business logic, tool definitions, or tenant awareness beyond resolving `tenant_id` |
| `web` | TypeScript | Onboarding dashboard, connector management UI, admin console, chat UI, embeddable widget | Never talks to a database directly — grpc-web through Envoy to `orchestrator`/`core` |

---

## 2. Request lifecycle

```mermaid
sequenceDiagram
    participant U as User
    participant Ch as Channel
    participant Envoy as Envoy (grpc-web)
    participant O as orchestrator
    participant Pl as Planner
    participant Ag as Specialist Agent
    participant TA as Tool Assembly
    participant C as core
    participant MCP as Tenant MCP Server
    participant LLM as LLM Provider

    U->>Ch: "Can you check my order status?"
    Ch->>Envoy: ChatStream(message, api_key/JWT)
    Envoy->>O: gRPC ChatStream
    O->>C: ResolveTenant + ResolveBotProfile (gRPC)
    C-->>O: tenant_id, bot_profile, connector list, RBAC role
    O->>C: GetSession / GetPreferences
    C-->>O: session context
    O->>TA: assemble available tools for this bot profile
    TA->>MCP: tools/list (per registered connector, cached + refreshed)
    MCP-->>TA: tool schemas
    TA-->>O: filtered tool set (profile allowlist ∩ RBAC role)
    O->>Pl: run planner node
    Pl->>LLM: classify intent (structured output)
    LLM-->>Pl: {agent: order_status, tool: check_order_status}
    Pl->>Ag: route
    Ag->>MCP: tools/call check_order_status(order_id)
    MCP-->>Ag: order status (tenant's own system, never touches core's DB)
    Ag->>LLM: synthesize answer
    LLM-->>O: streamed tokens
    O-->>Envoy: gRPC server-streaming
    Envoy-->>Ch: grpc-web server-streaming
    Ch-->>U: streamed answer
```

The step that differs fundamentally from a hardcoded-tool-catalog design: **tool assembly happens per request**, not at deploy time. `orchestrator` never ships knowing "tenant X has a `check_order_status` tool" — it discovers that by calling `tools/list` on whatever connectors that tenant's active bot profile has registered, caches the manifest, and refreshes it periodically or on a registry-change event.

---

## 3. Dynamic tool assembly

```mermaid
flowchart TB
    Req["Incoming request: tenant_id, channel, caller role"] --> Resolve["core.ResolveBotProfile"]
    Resolve --> Profile["Active bot profile:<br/>persona, connectors[], roles_allowed[]"]
    Profile --> Loop["For each registered connector"]
    Loop --> Cache{Manifest cached<br/>and fresh?}
    Cache -->|yes| Use[Use cached tools/list]
    Cache -->|no| Fetch["MCP initialize → tools/list"]
    Fetch --> Store["Cache manifest in core (TTL)"]
    Use --> Filter
    Store --> Filter["Filter: role ∈ roles_allowed<br/>AND tool ∈ profile's allowlist"]
    Filter --> Final["Final tool set handed to the Planner as<br/>this turn's available functions"]
```

Failure handling: a connector that's down or times out is dropped from the tool set for that turn (with a trace event), not treated as a hard failure of the whole request — one tenant's misbehaving connector never blocks their other connectors or any other tenant.

**Tool descriptions are mandatory, not optional metadata.** A tool name and JSON schema alone are not enough context for the planner/agent to use a tool correctly or interpret its result — the description is load-bearing, not decorative:

- **At registration/manifest-refresh time** (`core`): every tool returned by a connector's `tools/list` must carry a non-empty `description`. `core` rejects caching a manifest that has a tool missing one — a connector is only "active" once every tool it exposes is fully described. This is enforced in code today by `core/mcpclient.ListTools` / `RefreshManifest` (Phase 1).
- **At tool-call time** (`orchestrator`, Phase 3 — not yet built): a tool's cached description travels alongside its `tools/call` result when handed back to the agent/LLM, not just the raw result payload. The model reasons about a tool's output in light of what the tool claims to do, so the description is part of the result's context, every time — never dropped after the initial planning step. **Tracked as an explicit Phase 3 definition-of-done item in `PLAN.md`.**
- **For tenants/SDK integrators**: whoever stands up a connector (hand-written MCP server, or scaffolded via the future `connector-sdk`) is responsible for writing a complete, accurate description for every tool and resource they expose — not just a name. `core` treats a missing description as a registration-time validation failure, not a warning, precisely so this can't be skipped by an integrator in a hurry.

**Multi-agent supervisor** (`orchestrator/server/router.py`): before tool assembly's filtered set is offered to the planner, a lightweight classification step decides which specialist agent handles the turn — a **tools agent** (the tenant's own registered connectors/`HttpTool`s), a **web agent** (a built-in public web-search tool, no tenant data involved, no API key required — gated per bot profile by `BotProfile.web_search_enabled`, off by default since it sends the user's message to a public search engine), or an **analytics agent** (the tenant's own `HttpTool`s tagged `category="analytics"`). Each route offers a disjoint tool set for that turn, not a merged one. A route is only ever offered to the classifier if something real backs it this turn (`has_analytics_tools`/`web_search_enabled`) — no LLM call happens at all when "tools" is the only possible answer.

**Per-tool visibility** (`HttpTool.visibility`, `"internal"|"external"`, defaults to `"internal"`): lets one tenant expose a subset of their tools to customer-facing (`external`) bot profiles while keeping the rest staff-only, without running two separate connectors. Carried from `core` through mcp-gateway to orchestrator via MCP's standard `_meta` field on each `Tool` descriptor (`mcp-gateway/gateway/tenant_server.py`), since MCP's `Tool` schema has no first-class visibility concept — this is a Weave-specific extension, and a real (non-Weave) MCP server that doesn't set it is treated as `"external"` (the least restrictive default, so third-party connectors aren't silently hidden from external profiles). `orchestrator/tools/assembly.py` enforces the filter: an `external` bot profile only ever sees tools marked `external`; an `internal` profile sees every tool regardless, staff being trusted with the full surface by construction.

**Per-tool category** (`HttpTool.category`, `"general"|"analytics"`, defaults to `"general"`): the same `_meta` mechanism tags a tool as analytics-flavored so the router's analytics route has something real to offer instead of being a documented alias for "tools" (superseding the Phase 3.6-era alias — see `PLAN.md`'s notes on this phase for why the alias was replaced rather than kept alongside a real capability).

**Registering a subset of a business's APIs, not all of them.** `client.add_tool()` (`packages/weave-sdk`) registers one HTTP endpoint at a time — fine for a handful of tools, but a business with 70 existing routes that only wants 40 of them reasoned over by a bot shouldn't have to hand-write 40 calls. `client.add_tools_from_openapi()` (`packages/weave-sdk/weave/openapi.py`) is the bulk path: hand it the tenant's own OpenAPI 3.x document (their API's own descriptor — this repo has no MCP-hosted equivalent of a compiled proto descriptor, so an existing OpenAPI spec is the closest thing most businesses already have) plus an `include` or `exclude` set of `operationId`s, and it resolves to exactly the same per-tool decisions `add_tool()` would need made by hand: which 40 (or which 30 to drop), each with a `visibility`/`category` (overridable per operation via the spec's own `x-weave-visibility`/`x-weave-category` extension keys, defaulting spec-wide otherwise), and every one still required to carry a `description`/`summary` (enforced at registration time — `OpenApiRegistrationError`, not a later, harder-to-trace `core` rejection). `include`/`exclude` are deliberately mutually exclusive: state either the 40 to register or the 30 to skip, never both, so there's one unambiguous read of which subset an integrator meant. An operation named in neither `include` nor `exclude` (or simply never in the spec at all) is registered via neither path — equivalent to it not existing from Weave's point of view, same as the one-by-one case; nothing needs to be locked down on the business's own API, since it never entered the tool registry. Each registered tool's `description` is the string the planner reasons from (see "Tool descriptions are mandatory" above) — the same discipline applies whether a tenant registers 8 endpoints by hand or 40 from a spec.

**Per-bot-profile prompt** (`BotProfile.persona`): the literal system-prompt text for one bot profile, set via `create_bot_profile(persona=...)` and used verbatim as `orchestrator/server/chat_service.py`'s `_build_system_prompt` base prompt every turn — not a template, not a path to a tenant-owned file (orchestrator never reads tenant files; see the field's proto comment in `protos/database/v1/bot_profile.proto`). This is where a tenant states this bot's role, tone, and task scope beyond what tool descriptions alone convey, e.g. "You are Suvidha FinServe's staff-facing assistant. Prioritize accuracy over speed; if a figure isn't in a tool result, say so rather than estimating." A tenant with more than one bot profile (the common case — see §4's `bot_profiles` example) writes a separate persona per profile, since an external customer-facing bot and an internal staff-facing bot usually want different tones and scopes even when they share the same underlying tool registry. Left `""`, a profile falls back to a generic default rather than an empty prompt.

**Per-bot-profile guardrails** (`BotProfile.guardrails`): free-text disclosure rules, one bot profile's own list, e.g. `["Never disclose supplier names.", "Never disclose another customer's contact details."]` — enforced only when `visibility == "external"` (`docs/architecture/SECURITY.md` §6), checked against both the final answer and any tool result before it enters the model's context (`orchestrator/server/guardrails.py`, `server/graph.py`'s `_tool_node`). Exclusive to the profile that declares them: an `external` and `internal` profile on the same tenant typically carry entirely different guardrail lists (or none, for the internal one, since it isn't screened at all) rather than sharing one — set per `create_bot_profile()` call, same as `persona`.

**Per-bot-profile LLM provider** (`BotProfile.llm_provider`/`llm_model`): which model backend generates that profile's turns — the model-side counterpart to mcp-gateway's tool-side switch (§1). `orchestrator/llm/router.py`'s `get_provider()` resolves `llm_provider` (`""`/`"ollama"` → `llm/ollama_client.py`, orchestrator's local-model default; `"openai"` → `llm/openai_compat_client.py`, speaking the OpenAI chat-completions wire format — which covers OpenAI itself and any other backend exposing that same shape, since which one is determined by orchestrator's own `OPENAI_BASE_URL` configuration, not by a value a tenant sets) to a module; `server/graph.py`'s tool-decision call and `server/chat_service.py`'s synthesis call both resolve the same provider once per turn and call it uniformly, never branching on which one it is. Every provider module exposes the identical `chat()`/`chat_stream()` shape, so adding a third provider means adding a module, not touching either caller. `llm_model` names the model to request from whichever provider that resolves to (e.g. `"llama3.2:3b"`, `"gpt-4o-mini"`), falling back to that provider's own configured default when left `""`. Credentials for a non-default provider (e.g. `OPENAI_API_KEY`) are orchestrator's own configuration — the same trust boundary as `OLLAMA_HOST` — a bot profile picks which already-configured backend to use, it doesn't supply its own API key through the platform; per-tenant credential-vault integration for that is real future work, not implemented here.

**Per-tool end-user auth** (`HttpTool.auth_mode`, `""`/`"none"` (default) | `"user_token"`): for a tenant endpoint that must be scoped to the *specific signed-in Weave user* asking — a finance app's "my own transactions," not just any authorized caller. Every `ChatStream` turn is already authenticated (§1's "no anonymous path"), so `"user_token"` only ever restricts a tool to real, registered users, never opens one up. Mechanism, both hops entirely within Weave's own trust zone until the final outbound call:

1. `orchestrator/server/chat_service.py` mints a short-lived `{tenant_id, user_id}` JWT once per turn (`server/auth.py`'s `mint_user_assertion`, signed with the same `JWT_SECRET` `core` and `orchestrator` already share) and forwards it on every tool call that turn makes, via MCP's standard `tools/call` `_meta` field (`mcp_client/client.py`) — the same open-extension mechanism `_meta` already carries `visibility`/`category` on `tools/list`, just on the call side. Sent on every call regardless of the target tool's `auth_mode`; harmless, since only mcp-gateway ever acts on it.
2. `mcp-gateway/gateway/tenant_server.py` checks the *target tool's* `auth_mode`. For `"none"` (the default), nothing changes — `credential_ref_id` (if any) still goes out as a static `Authorization: Bearer` header, same as always. For `"user_token"`: it verifies the assertion (`gateway/user_assertion.py`, checking signature, expiry, and that the token's `tenant_id` matches the tenant this MCP server instance is already scoped to — refusing the call outright if missing or invalid), then reinterprets `credential_ref_id` as an **HMAC-SHA256 signing key** rather than a bearer token and computes `signature = HMAC-SHA256(secret, f"{tenant_id}:{user_id}")` (`gateway/http_signing.py`), sending `X-Weave-User-Id` / `X-Weave-Tenant-Id` / `X-Weave-User-Signature` to the tenant's real endpoint instead of an `Authorization` header.
3. The tenant's own API verifies that signature with the same secret it registered at `add_tool(..., credential_secret=..., auth_mode="user_token")` time — the same well-known webhook-signing-secret pattern Stripe/GitHub use for verifying inbound requests — then trusts `X-Weave-User-Id` and scopes its response to that user however it maps Weave users onto its own user records. Weave never needs to know that mapping.

`core` rejects registering a `"user_token"` tool with no `credential_secret` at write time (nothing to sign with, so the tenant could never verify anything) rather than letting it fail silently on first call.

---

## 4. Data model (owned by `core`)

Weave's own MongoDB holds only platform data — never a tenant's business records.

```mermaid
erDiagram
    TENANTS ||--o{ BOT_PROFILES : has
    TENANTS ||--o{ CONNECTORS : registers
    CONNECTORS ||--o{ CREDENTIAL_REFS : "auth via"
    TENANTS ||--o{ USERS : has
    USERS ||--o{ CHAT_SESSIONS : starts
    CHAT_SESSIONS ||--o{ CHAT_MESSAGES : contains
    USERS ||--o{ USER_MEMORIES : has
    TENANTS ||--o{ USAGE_RECORDS : accrues
```

Representative documents (`_id` is a string ULID):

```jsonc
// connectors
{ "_id": "conn_01H…", "tenant_id": "acme-clinic", "name": "acme-booking-mcp",
  "transport": "http+sse", "endpoint": "https://acme.example.com/mcp",
  "credential_ref": "cred_01H…", "capability_manifest": { /* cached tools/list */ },
  "manifest_refreshed_at": "2026-08-11T10:00:00Z", "status": "healthy" }

// bot_profiles
{ "_id": "profile_01H…", "tenant_id": "acme-clinic", "name": "external",
  "persona": "You are Acme Clinic's booking assistant. Be warm and concise;
    never discuss another patient's appointments.",
  "guardrails": ["Never disclose a patient's diagnosis or medication.",
    "Never disclose another patient's contact details."],
  "connector_ids": ["conn_01H…"],
  "channels": ["web-widget", "whatsapp"], "roles_allowed": ["customer"] }

// chat_messages
{ "_id": "msg_…", "tenant_id": "acme-clinic", "session_id": "ses_…",
  "role": "assistant", "content": "…", "tool_calls": [ /* structured, incl. which connector */ ],
  "created_at": "…" }
```

Every collection is tenant-scoped (`tenant_id` field + compound index), same as the trust-boundary rule below.

---

## 5. RAG and memory

- **RAG sources are pluralized.** A tenant's knowledge base can be Weave-hosted documents *or* MCP resources exposed by their own systems (a wiki, a doc store) — the ingestion pipeline pulls from either, chunks, embeds, and writes to a **per-tenant Qdrant collection**. Not yet built.
- **Memory has two tiers, both written and read only through `core`, never held by `orchestrator` directly**:
  - **Session memory** (`core.ChatService`, Mongo-backed `chat_sessions`/`chat_messages`): every turn's user/assistant messages, scoped to one conversation. `orchestrator/server/session_memory.py` loads prior turns before generating and appends both sides after — real multi-turn context, not per-call statelessness. Fails soft on any `core` error (a turn degrades to "no prior context," never fails outright).
  - **Long-term/semantic memory** (`core.MemoryService`, Qdrant-backed, one collection per tenant named `mem_{tenant_id}` — same isolation decision as RAG above, and the same collection additionally filters by `user_id` in each point's payload so one user's memories never surface for another user of the same tenant). `orchestrator` computes the embedding (it holds LLM/embedding-model access; `core` never runs inference) via Ollama's embedding endpoint and hands `core` the vector — `core` remains the only tier holding the Qdrant connection itself. Qdrant point ids are UUIDs (a real constraint of the store), not this codebase's usual ULID convention.

```mermaid
sequenceDiagram
    participant O as orchestrator
    participant Ollama as Ollama (embed)
    participant C as core
    participant Q as Qdrant

    O->>Ollama: embed(user message)
    Ollama-->>O: vector
    O->>C: MemoryService.SearchMemory(tenant_id, user_id, vector)
    C->>Q: Query(collection=mem_{tenant_id}, filter user_id)
    Q-->>C: top-k matches
    C-->>O: results
    Note over O: relevant facts spliced into<br/>this turn's system prompt
    O->>Ollama: embed(user message) (for storage)
    O->>C: MemoryService.UpsertMemory(tenant_id, user_id, text, vector)
    C->>Q: Upsert point
```

---

## 6. Observability

Every planner decision, tool call, and MCP round trip is a traced span (Langfuse), tagged with `tenant_id`, `bot_profile`, `session_id`, and — critically — which **connector** served a given tool call, so a tenant's own MCP server's latency/errors are visible and attributable, not blended into Weave's own service metrics. OpenTelemetry covers infra-level RPC latency/error rate across `core`, `orchestrator`, and `compute`.

---

## 7. Deployment

Local: Podman Compose brings up Mongo, Redis, Qdrant, MinIO, Envoy, `core`, `orchestrator`, `compute`, and `web`. Production: Kubernetes, one Deployment + Service per service, HPA on `orchestrator` and `core` at minimum. Tenant-owned MCP servers are **never** deployed by Weave's own infrastructure — they live wherever the tenant hosts them, reached over the network like any external API.

---

## 8. `web`

Next.js 14 (App Router) + TypeScript + Tailwind + shadcn/ui, per `OVERVIEW.md` §5's committed stack. Two authenticated areas, both behind `RequireAuth`/`AdminLayout` guards backed by a client-side `AuthProvider` (JWT + tenant_id held in `localStorage`, never trusted server-side beyond what the JWT itself proves — same "never a client-supplied tenant_id" rule as everywhere else, `SECURITY.md` §2):

- **`/chat`**: the customer/staff-facing chat experience — channel picker, streaming message thread, tool-used badges, "New chat" (drops the client-held `session_id`, letting `orchestrator` start a fresh one per §5's session memory).
- **`/admin`**: tenant overview, bot-profile management (list + create, including guardrails/web-search/visibility), a read-only tools registry (visibility/category at a glance), and connectors — everything a business needs to see and shape without touching `core` directly.

**No hand-written gRPC client code**: `web/buf.gen.yaml` runs `@bufbuild/protoc-gen-es` (target `ts`) over the same `protos/` tree Go/Python codegen uses, producing typed message/service descriptors in `src/gen/`; `@connectrpc/connect-web`'s `createGrpcWebTransport` + `createClient` turn those into real RPC clients — the same source of truth as every other language, not a hand-maintained REST shim.

**Envoy is required and non-optional** — the browser cannot speak raw gRPC (`ChatStream` is a real server-streaming gRPC call), only grpc-web, which Envoy's `envoy.grpc_web` filter translates before forwarding to `orchestrator` (path prefix `/orchestrator.v1.`) or `core` (everything else). `infra/envoy/envoy.yaml` is the container-shaped config matching `podman-compose.yml`'s `envoy` service. A second file, `infra/envoy/envoy.local.yaml`, exists purely for a local-dev networking quirk (documented in that file's own comment): running Envoy as a native binary (e.g. via `func-e`) instead of in a container, because container-to-host-process routing didn't work reliably in this project's Windows/WSL2 podman setup even though DNS resolution (`host.containers.internal`) succeeded — the same class of issue `PLAN.md`'s earlier phases hit with podman networking, not a new architectural decision.
