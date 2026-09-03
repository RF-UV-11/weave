# Weave — Build Plan

How to use this file: work top to bottom, don't start a phase until the previous one's "Definition of done" is met. Tick boxes as you go; add a one-line note under a phase if you hit a real gotcha. Full rationale for *why* things are shaped this way lives in `docs/architecture/ARCHITECTURE.md` and `docs/architecture/SECURITY.md` — this file is the *what/when*.

**The trust boundary that governs every phase**: `core` (Go) is the only tier holding Weave's own database credentials, and it only ever holds platform data (tenants, connectors, credentials, chat/memory, auth, billing) — never a tenant's business data. See `OVERVIEW.md` and `docs/architecture/SECURITY.md` before touching `core`.

---

## Phase 0 — Contracts & Platform Data Tier

**Goal**: the spine — a real `protos/` contract, generated Go stubs, and a `core` Go service writing to MongoDB. Nothing else touches the DB.

- [x] `protos/buf.yaml` + `protos/buf.gen.yaml` — codegen to Go (`core/gen`)
- [x] First contracts: `protos/database/v1/common.proto` (Page, Health, ErrorDetail), `protos/database/v1/tenant.proto` (`Tenant`, `is_collection: true`)
- [x] `protos/core/data_access/v1/tenant.proto` — `CreateTenant`, `GetTenant`, `ListTenants` (cursor-paginated)
- [x] `core/` Go module: `mongodb/` (initialize.go, collections.go, indexes.go, health.go, tenant.go — the only Mongo access), `rpc_services/tenant/` (server.go + routehandler.go), `configs/`, `main.go` at module root (gRPC server + reflection + `Health` RPC)
- [x] `infra/podman-compose.yml` — mongo, redis:7, qdrant/qdrant, minio/minio, healthchecks, plus `core`
- [x] Confirm `core`'s `CreateTenant` over gRPC writes a real Mongo document, verified via `grpcurl` + `mongosh`

**Definition of done**: ✅ **met.** `grpcurl … CreateTenant` inserted `tnt_01KZRC2XWM6ZWYSC0BP0GE57BP`, confirmed via `mongosh` (`{"_id":"tnt_…","display_name":"Acme Clinic","tenant_type":"business","created_at":"…"}`) and re-read via `GetTenant`.

**Notes**:
- `_id` convention carried forward from the reference design: every `is_collection: true` entity's first field is `_id` (Mongo's real primary key), which protoc-gen-go renders as `XId`/`GetXId()` for the leading underscore — expected, not a bug. `mongodb.InitDatabase` sets `SetBSONOptions(&options.BSONOptions{UseJSONStructTags: true})` so the generated struct's `json:"_id,..."` tags drive Mongo (de)serialization directly — no separate DTO layer.
- Mongo driver: `go.mongodb.org/mongo-driver/v2`.
- **Windows/podman-compose networking gotcha (recurring from before)**: on this Windows/WSL2 podman-machine setup, a freshly `podman run`'s published port is not reliably reachable from the Windows host over `127.0.0.1` even though `podman ps`/`inspect` report the mapping correctly (confirmed working *inside* the podman machine VM via `podman machine ssh`, not from the host). Verified Phase 0 by attaching both `mongo` and `core` to a user-defined `podman network create weave-net` and using container-DNS (`mongo:27017`) instead of relying on host port-forwarding for service-to-service traffic — only `core`'s own published port (for external `grpcurl` access) needs to cross the host boundary. `infra/podman-compose.yml` creates its own network for services automatically so this should be transparent under real `podman-compose up`, but if `up` ever hangs on Mongo health, check host-port reachability with `podman machine ssh -- curl 127.0.0.1:<port>` before assuming Mongo itself is broken.

---

## Phase 1 — Connector Registry & Credential Vault

**Goal**: the mechanism that makes Weave "plug-and-play" — tenants can register an MCP connector and its credentials, safely.

- [x] `protos/database/v1/connector.proto` (`Connector`, `is_collection: true`: tenant_id, name, transport, endpoint, credential_ref, capability_manifest, status)
- [x] `protos/database/v1/credential.proto` (`CredentialRef`, `is_collection: true` — reference only; see `docs/architecture/SECURITY.md` §3 for the vault design decision)
- [x] `protos/core/data_access/v1/connector.proto` — `RegisterConnector`, `ListConnectors`, `RefreshManifest`, `DeregisterConnector`
- [x] **Design spike, before writing vault code**: external vault (HashiCorp Vault/cloud KMS) vs. app-level envelope encryption — decided app-level envelope encryption, documented in `docs/architecture/SECURITY.md` §3
- [x] `core/mongodb/connector.go` + `core/rpc_services/connector/`
- [x] Isolation test: two tenants register connectors with colliding names, confirm no cross-tenant leak in `ListConnectors`

**Definition of done**: ✅ **met.** Registered `acme-booking-mcp` (tenant `tnt_01KZRE1R2R42H3Y80Q02YMZ7K7`) against `connectors/dev-stub-mcp` (trivial HTTP JSON-RPC stub), `RefreshManifest` cached its real `tools/list` result (`book_appointment`) onto the connector doc and flipped `status` to `active`. `mongosh` confirmed the stored `CredentialRef` holds only `ciphertext`/`nonce`/`wrapped_dek`/`dek_nonce` — no plaintext secret. A second tenant (`tnt_01KZREDYRMX40P947XFA38DV7G`) registered a connector with the same name `acme-booking-mcp`; each tenant's `ListConnectors` only ever returned its own.

**Notes**:
- `core/vault/vault.go` — AES-256-GCM envelope encryption: a random per-credential DEK encrypts the secret, then gets wrapped by a root key from `VAULT_ROOT_KEY` (base64, 32 bytes) that's only ever held in memory. Root key rotation and per-access audit logging are explicitly deferred — see `docs/architecture/SECURITY.md` §3's "known gap" note; must land before any real (non-dev) tenant credential is stored.
- `core/mcpclient/client.go` is a deliberately minimal MCP slice (HTTP-only `tools/list` JSON-RPC call, 1 MiB response cap) just for `core` to cache a manifest — it is not the real MCP client, which is Phase 3's `orchestrator` scope (stdio/SSE transports, `initialize`, `tools/call`).
- `connectors/dev-stub-mcp/` — throwaway Python stdlib HTTP stub used only to exercise `RefreshManifest` in dev; not a real connector template.
- **Podman networking, same gotcha as Phase 0**: `host.containers.internal` did not resolve on the user-defined `weave-net` bridge network in this WSL2/podman-machine setup (confirmed via `podman run --network weave-net busybox nslookup host.containers.internal` → NXDOMAIN). Running the stub MCP server as its own container on `weave-net` (reached via container DNS, same pattern as `weave-mongo`) worked immediately — prefer that over host-port tricks for anything `core` itself needs to reach.

---

## Phase 2 — Bot Profiles & Auth

**Goal**: named bot profiles per tenant, JWT + RBAC wired through.

- [x] `protos/database/v1/bot_profile.proto`, `protos/database/v1/auth.proto` (`User`, `Role`)
- [x] `protos/core/data_access/v1/auth.proto` — `Register`, `Login`, `Refresh`; `protos/core/data_access/v1/bot_profile.proto` — `CreateBotProfile`, `GetActiveBotProfile(tenant_id, channel)`
- [x] `packages/shared-auth` (Go) — JWT-verify interceptor + `requires_role(...)`, tenant-scoped
- [x] `core/mongodb/{auth,bot_profile}.go` + rpc services

**Definition of done**: ✅ **met.** Seeded tenant `tnt_01KZRJW44K1RB8DJCPFEYQAPAE`, registered an owner user, logged in for an access token, created `external` (channels `[web-widget, whatsapp]`, `roles_allowed: [customer]`) and `internal` (channels `[slack]`, `roles_allowed: [staff, admin]`) bot profiles. `GetActiveBotProfile` on `web-widget` correctly resolved `external`, on `slack` correctly resolved `internal`. Also verified the auth gate itself: no token → `Unauthenticated`; an owner token scoped to a different tenant → `PermissionDenied` ("token is not scoped to this tenant"); a `customer`-role token calling `CreateBotProfile` → `PermissionDenied` ("role is not permitted").

**Notes**:
- `packages/shared-auth` is its own Go module (not nested under `core/`) since `packages/` is meant to be reused across Weave's services, Go or not. `core/go.mod` pulls it in via a `replace` directive to a relative path; both Containerfiles now copy `packages/shared-auth` into the build context at the matching relative layout (`/repo/packages/shared-auth` alongside `/repo/core`) so the replace resolves inside the container too.
- The JWT interceptor is wired **server-wide** on `core`'s `grpc.Server` (`grpc.UnaryInterceptor`), with an explicit skip list in `main.go` for: the auth bootstrap RPCs (can't require a token to log in), health/reflection (so `grpcurl` and infra healthchecks keep working), and every pre-Phase-2 Tenant/Connector RPC. Those predate auth and aren't retrofitted with tenant/role checks yet — **known gap, tracked as a follow-up**: lock down Tenant/Connector RPCs the same way BotProfileService already is, once there's a clear per-RPC role policy for them (e.g. who's allowed to `RegisterConnector`).
- Refresh tokens are stateless JWTs (7-day TTL) — same "ship the mechanism, flag the operational gap" pattern as the Phase 1 vault. No revocation/rotation store yet, so a leaked refresh token is valid until it naturally expires; not acceptable before real tenant credentials are on the line, tracked in `docs/architecture/SECURITY.md` alongside the vault's own known gaps.
- Caught in verification, not design: `Register`/`Login` were echoing the bcrypt `password_hash` back in the response `User` — harmless (not reversible) but needless exposure, fixed by redacting it before the RPC boundary.
- Found and fixed a real flake in the *existing* test suite while adding Phase 2 tests: `go test ./...` runs packages concurrently, and every package's `TestMain` was connecting to the same `weave_core_test` Mongo database — one package's teardown `Drop()` could wipe another package's still-running data. Each package now uses its own database name (`weave_core_test_mongodb`, `_connector`, `_tenant`, `_auth`, `_bot_profile`).

---

## Cross-cutting — Rate limiting & abuse defense

**Goal**: every RPC on `core` has a DDoS/brute-force/abuse defense baseline, independent of and ahead of auth. Not a phase in the tenant-facing feature sense — a security requirement applied across whatever phase is currently landing.

- [x] `packages/shared-ratelimit` (Go) — Redis-backed fixed-window limiter + gRPC interceptor, per-method limits, fails open on Redis outage
- [x] Wired into `core/main.go` via `grpc.ChainUnaryInterceptor`, ahead of the auth interceptor (unauthenticated RPCs like `Login` need protection too)
- [x] Tight limits on `Login`/`Register` (brute-force/spam targets), generous default elsewhere, health/reflection exempt

**Definition of done**: ✅ **met.** Hammered `Login` with 7 back-to-back `grpcurl` calls from the same client: the first 5 succeeded (returning the expected `Unauthenticated` for bad credentials), the 6th and 7th returned `ResourceExhausted`. Confirmed `CreateTenant` (a different method) was unaffected by `Login`'s exhausted budget — limits are per-method, not global.

**Notes**:
- Full design rationale lives in `docs/architecture/SECURITY.md` §5, including the deliberate choice not to trust a client-supplied `x-forwarded-for` header.
- **Real bug caught in live verification, not by unit tests**: the first implementation keyed on the full peer address (`ip:port`). Every gRPC call is its own TCP connection with a fresh ephemeral source port, so that key gave every single request a "new" bucket — rate limiting was silently a no-op end-to-end despite 10 passing unit tests, because every test reused one hardcoded port across all calls. Fixed by stripping the port (`net.SplitHostPort`), and added a regression test that varies the port across calls to catch this class of bug going forward. Lesson: a passing unit-test suite for a network-keyed system doesn't substitute for testing against real, varying connections.
- This is distinct from a future tenant-plan usage quota (business rule, keyed by `tenant_id`, layered on top once identity is known) — see the Parking lot / productization discussion below.

---

## Cross-cutting — Health checks & local Kubernetes prototype (minikube)

**Goal**: real health checks (not a static "always up"), and `core` + its dependencies actually running on Kubernetes — the first step toward "deploy this and offer it as a service" rather than only ever running under `podman-compose`.

- [x] `core`'s gRPC health status (`grpc.health.v1.Health`) reflects real Mongo/Redis connectivity via a background loop (`core/health.go`), not a value set once at startup and never updated
- [x] `packages/shared-ratelimit` gets a `Ping` method so the health loop can check Redis the same way `mongodb.Healthy` already tracks Mongo
- [x] `infra/k8s/{namespace,mongo,redis,core-secret,core}.yaml` — Mongo as a `StatefulSet` with a `PersistentVolumeClaim`, Redis as a plain `Deployment` (its data is TTL-bound rate-limit counters, fine to lose on restart), `core` as a `Deployment` with `grpc` readiness/liveness probes against the now-real health status
- [x] `grpc-health-probe` added to `core`'s distroless image (`go install`, since there's no shell to run a `CMD-SHELL` script) and wired as `core`'s `podman-compose.yml` healthcheck — previously the only service in that file without one
- [x] Liveness probes added to the k8s `mongo`/`redis` manifests (they only had readiness before)

**Definition of done**: ✅ **met**, on minikube with the podman driver. Deployed all four workloads, confirmed `grpcurl` through `kubectl port-forward` reaches all four services and a `CreateTenant` write actually lands in the StatefulSet-backed Mongo (`db.tenants.findOne()` inside the `mongo-0` pod shows the record). Confirmed rate limiting works against the in-cluster Redis the same way it did under `podman-compose`. Confirmed the health check is *real*, not decorative: stopped the Mongo container directly, watched `core`'s health flip to `NOT_SERVING` within ~20s via `grpc-health-probe`, then watched it recover once Mongo came back — this is what makes the k8s `livenessProbe`/`readinessProbe` on `core` meaningful instead of just checking the process is alive.

**Notes**:
- **Podman driver + Windows/WSL2 gotcha, worse than the earlier networking ones**: under the *rootless* podman machine (the mode this project had been using throughout Phases 0–2), minikube's pod network was completely broken — not just service DNS, but raw pod-to-pod TCP by IP address timed out too (confirmed with a `busybox` debug pod: `nc -zv <pod-ip> <port>` timed out even bypassing DNS entirely). Switching the podman machine to **rootful** (`podman machine set --rootful`) fixed it outright. This is a real behavioral difference, not a config tweak — rootful and rootless podman have entirely separate container/image namespaces, so switching made the pre-existing `podman-compose` dev containers (`weave-mongo`, `weave-core`, `weave-net`, etc.) disappear from `podman ps` (recoverable by switching back to rootless; nothing was deleted). **Decision: the podman machine stays rootful going forward** — minikube needs it, and the `podman-compose` dev workflow works identically under rootful, just under a different container namespace than whatever was running before this session.
- **Getting an image into minikube**: `minikube image load <name>` tried to resolve the image via a Docker daemon that isn't installed on this machine and failed outright. `podman save -o image.tar <name>` followed by `minikube image load image.tar` works reliably — prefer the tarball path over the by-name path on a podman-only setup.
- `grpc-health-probe` is installed via `go install github.com/grpc-ecosystem/grpc-health-probe@v0.4.28` in the build stage rather than downloading a release binary by URL/arch — it's a Go module, so this is more reproducible and doesn't need `curl`/`wget` in the alpine build image.
- Secrets in `infra/k8s/core-secret.yaml` are the same dev-only values already committed in `podman-compose.yml` — fine for a throwaway local prototype, **not** fine once this stops being one. See `docs/architecture/SECURITY.md` §3's vault known-gaps note for the same caveat applied to `VAULT_ROOT_KEY`.
- Deliberately did **not** stand up Qdrant/MinIO in k8s yet — nothing in `core` uses them today (only Mongo and Redis have real code paths against them). Add them when `orchestrator`/RAG actually needs them (Phase 3+), not preemptively.
- This is a **local prototype** (minikube), not a real deployment target — no ingress, no TLS, no real secrets management, single replica of everything, no autoscaling. `PLAN.md`'s Phase 4+ already earmarks real Kubernetes deployment as a later concern; this cross-cutting item is "prove it can run on k8s at all and health-checks work," not that milestone.

---

## Phase 3 — Orchestrator Core + MCP Client

**Goal**: `orchestrator` exists, streams a real LLM response, and can call a real MCP connector via dynamic tool assembly.

- [x] `protos/orchestrator/v1/chat.proto` — server-streaming `ChatStream`; Python codegen via each service's own `initialize.sh` (not a `protos/`-owned script — `protos/` stays proto-only)
- [x] `orchestrator/` Python project: `server/chat_service.py`, `llm/` (Ollama), `mcp_client/` (initialize → tools/list → tools/call)
- [x] `orchestrator/tools/assembly.py` — the dynamic tool-assembly mechanism from `docs/architecture/ARCHITECTURE.md` §3
- [x] `packages/shared-clients` (Python) — generated gRPC client to `core`
- [x] Reference connector: `connectors/reference-mcp/` — a real MCP server (fake booking system) to develop against
- [x] Minimal dev UI: `orchestrator/dev_cli.py` (CLI harness, not the real `web` app)

**Definition of done**: ✅ **met.** Registered a connector against `connectors/reference-mcp`, created an `external` bot profile, asked the dev harness "book me an appointment for 2026-08-20 at 3pm, my name is Ada" via `dev_cli.py`. Watched `book_appointment` get discovered dynamically (via a live MCP `tools/list`, not a hardcoded catalog), called, and the real result — `bkg_1: Ada on 2026-08-20 at 15:00` — stream back token by token; confirmed the booking actually persisted by calling `list_appointments` independently afterward. Separately confirmed a question needing no tool ("what is 12 plus 30?") also streams correctly, and that an unauthenticated `ChatStream` call is rejected (`UNAUTHENTICATED`). All end-to-end through `core` for tenant/bot-profile resolution — no client-supplied `tenant_id` anywhere in the path, resolved entirely from the caller's JWT.
- [x] **Tool description carried through to tool-call results** (requirement added post-Phase-1): `server/graph.py`'s tool node embeds `{tool.description}\n\nResult: {result}` as the tool-role message content — the model (and, via `ChatStreamResponse.tool_used`, the caller) never sees a raw result without the tool's description alongside it.

**Notes**:
- **LLM backend**: installed Ollama (`winget`) + pulled `llama3.2:3b` (~2GB) rather than using a cloud API, matching OVERVIEW.md's stated "Ollama first" — fully local, no API key needed. `llm/ollama_client.py` is written behind a `chat()`/`chat_stream()` shape so a cloud provider can be a sibling module later without touching callers.
- **Streaming design, not obvious from the code alone**: the LangGraph graph (`server/graph.py`) *never* generates the user-visible answer — it only decides whether a tool is needed and, if so, calls it. The very first version of this did call the LLM a second time non-streaming for the "final" answer and then would have had to fake-stream it; that both wastes a generation and isn't real streaming. The current design calls the model with `tools` offered exactly once (a decision call, content discarded if no tool is chosen), executes at most one tool, then `chat_service.py` makes exactly one real streaming call on whatever message list the graph settled on. One decision call + at most one tool call + one streaming call, no wasted or duplicate generation, genuine token-level streaming in both the tool-used and no-tool-used paths.
- **A real networking asymmetry, not a bug in the usual sense**: `core` runs containerized (podman) while `orchestrator` runs on the host for Phase 3, so a single `connector.endpoint` string can't be simultaneously valid for both core's `RefreshManifest` (container-network addressing) and orchestrator's live MCP calls (host addressing) without extra infra (e.g. also containerizing orchestrator on the same network — not done yet). Resolved by having tool assembly **not** gate on `core`'s cached `connector.status` field at all — that field reflects core's own cache validity, a different concern from whether orchestrator can reach the connector right now. Orchestrator's own live `tools/list` call is the authoritative check, matching `ARCHITECTURE.md` §3's diagram (Tool Assembly calls MCP `tools/list` directly, not gated by a cache). Revisit once orchestrator is containerized and shares a network with `core`.
- **Bot-profile role gating happens in `orchestrator`, not `core`**: `SECURITY.md` §5 describes `roles_allowed` as a filter "on top of RBAC," and `ARCHITECTURE.md` §3's diagram places that filter inside Dynamic Tool Assembly — so `tools/assembly.py` checks the caller's JWT role against `profile.roles_allowed` itself; `core`'s `GetActiveBotProfile` RPC (Phase 2) only checks the token is scoped to the right tenant, not the role-vs-profile match. Consistent with the architecture doc, but worth knowing if you go looking for that check in `core` and don't find it.
- **Persona files aren't loaded yet**: `BotProfile.persona` is just a path reference (e.g. `"personas/external.md"`); `chat_service.py` uses one hardcoded `DEFAULT_SYSTEM_PROMPT` for every tenant/profile rather than reading that file. Tracked as a follow-up, not in Phase 3's explicit scope.
- Every service now owns its own `initialize.sh` (env → deps → proto compile → start) rather than `protos/` owning a shared codegen script — `orchestrator/initialize.sh`, `connectors/reference-mcp/initialize.sh`. `protos/` contains only `.proto` files and the Go-focused `buf.yaml`/`buf.gen.yaml`.

---

## Phase 3.5 — HTTP Tool Registry, mcp-gateway & the weave SDK

**Goal**: the actual product simplification — a business attaches an existing public HTTP API as a bot tool via `import weave`, without writing or hosting any MCP server themselves. Every tool call still goes over **real MCP protocol end-to-end** (explicit product decision — see below), it's just that Weave hosts the MCP server on the business's behalf instead of requiring them to.

- [x] `database.v1.HttpTool` + `core/data_access/v1.HttpToolService` (`RegisterHttpTool`, `ListHttpTools`, `DeregisterHttpTool`, `RevealHttpToolCredential`) — the "dedicated reveal path" `docs/architecture/SECURITY.md` §3 flagged as not yet existing back in Phase 1 now exists, gated as an internal-service-only RPC (known gap, same category as `ListConnectors`/`ListHttpTools` — no per-user JWT since the caller is trusted infra, not acting for an end user)
- [x] `core/mongodb/http_tool.go` — `GetOrCreateManagedConnector` auto-creates one `Connector` per tenant (`transport: "weave_managed"`) that every `HttpTool` a tenant registers belongs to, mirroring how a real MCP server groups multiple tools under one endpoint
- [x] `core/netguard` — SSRF guard on every tenant-supplied endpoint (`RegisterConnector` *and* `RegisterHttpTool`); see `docs/architecture/SECURITY.md` §4
- [x] `mcp-gateway/` — a new service: routes `/{tenant_id}/mcp` to a freshly-built per-tenant MCP server (`mcp.server.lowlevel.Server`) backed by that tenant's current `HttpTool`s, so `orchestrator`'s existing MCP client needs zero changes to consume it
- [x] `packages/weave-sdk` (`import weave`) — `weave.connect(tenant_id=, email=, password=)` returns a client with `add_tool()`/`list_tools()`/`remove_tool()`

**Definition of done**: ✅ **met.** `weave.connect(...)` + `client.add_tool(name="get_order_status", description=..., endpoint="http://.../orders/{order_id}/status", ...)` against a throwaway fake business API (`mcp-gateway/tests/fake_business_api.py`), then `orchestrator`'s `dev_cli.py` asked "What is the shipping status of order 123?" — watched `get_order_status` get discovered dynamically over real MCP protocol (not the raw HTTP tool definition — `orchestrator` never sees `HttpTool`, only a normal MCP `tools/list`/`tools/call`), the gateway substitute `{order_id}` into the real endpoint's path, call the fake API, and the real result (`"shipped"`, ETA `2026-08-20`) stream back correctly. Confirmed the no-tool-needed path still works, and that a different tenant sees zero tools (isolation).

**Notes** — several real bugs found and fixed before this worked, worth keeping:
- **`add_tool()`'s schema can't come from Python function introspection.** The high-level `MCPServer.add_tool(fn, ...)` API (what `connectors/reference-mcp` uses) derives a tool's JSON schema from the registered Python function's own type-annotated signature — but `HttpTool.params_schema` is an arbitrary business-supplied JSON Schema string with no corresponding Python function to introspect. Building `mcp-gateway` on the high-level API would have required synthesizing fake function signatures per tool. Used `mcp.server.lowlevel.Server` instead (`on_list_tools`/`on_call_tool` callbacks returning explicit `types.Tool(inputSchema=...)`/`types.CallToolResult` objects), which takes the schema directly.
- **A fresh-per-request MCP sub-app needs its ASGI lifespan driven manually.** `streamable_http_app()`'s session manager requires a real `lifespan.startup` event before it'll handle any request ("Task group is not initialized. Make sure to use run()." otherwise) — a dispatcher that only forwards the raw HTTP request (no lifespan) breaks immediately. `mcp-gateway/server.py`'s `_start_lifespan()` drives just enough of the ASGI lifespan protocol per freshly-built sub-app to satisfy this.
- **Event-loop affinity bit twice, independently, in two different places** — the same underlying mistake (a `grpc.aio` channel is bound to whichever event loop was running when it was constructed) surfaced twice: (1) `weave-sdk`'s first `SyncWeaveClient` draft called `asyncio.run()` per method, tearing down and recreating the loop every call — broke on the *second* call, not the first, since the channel from call 1 doesn't survive into call 2's fresh loop. Fixed with a persistent background thread running one event loop for the client's lifetime (`run_coroutine_threadsafe` per call), covered by a regression test that calls `add_tool()` twice. (2) `mcp-gateway/server.py`'s `CoreClient` was originally constructed at **module import time** — which happens before `uvicorn.run()` creates its own loop — so the very first RPC failed with "attached to a different loop." Fixed by deferring construction into the real ASGI lifespan startup handler, which runs inside uvicorn's actual loop. Lesson: any `grpc.aio`/asyncio object that needs to outlive a single call must be created *inside* the loop that will use it, never at import time or via a fresh `asyncio.run()` per call.
- **The managed connector's endpoint needs the transport's mount path, not just a tenant-scoped base URL.** First version stored `gatewayBaseURL + "/" + tenantID` — 404 immediately, since `mcp-gateway` strips the `/{tenant_id}` prefix and forwards the remainder to a per-tenant sub-app whose default mount path is `/mcp`. Caught in live verification (grpcurl + a real gateway request), not by unit tests — none of `core/mongodb/http_tool_test.go`'s or the RPC layer's tests actually round-tripped through a running gateway, only asserted the endpoint *string* was well-formed. Fixed by appending `/mcp`; both test files updated to assert the corrected shape, but this is a reminder that "the string looks right" isn't the same as "a real client can reach it."
- **"Always require real MCP," not a shortcut bypassing it** — this was an explicit product decision (see conversation), not a default: the alternative (core/orchestrator calling a business's HTTP endpoint directly, skipping MCP for the simple case) was considered and rejected in favor of `mcp-gateway` making every tool call, including these, genuine MCP protocol on the wire. `orchestrator`'s MCP client required zero changes to support this.
- `mcp-gateway` rebuilds a tenant's server (and re-fetches their `HttpTool`s, and re-reveals credentials) on **every** request — no caching. Correct (a tool registered a moment ago is usable immediately) but adds a `core` round trip per MCP call; a short-TTL cache is a straightforward follow-up if the latency ever matters.
- `weave-sdk`'s `connect()` uses tenant_id + email + password (Phase 2's actual auth model) — the pitch example used `api_key=`, which doesn't exist yet. Real API-key auth for programmatic/SDK access is a natural follow-up, not built here.
- Credential auth convention for a business's HTTP endpoint is fixed at `Authorization: Bearer <secret>` (`mcp-gateway/gateway/http_caller.py`) — no support yet for API-key headers, basic auth, or query-param keys. Documented as a v1 limitation, not silently assumed away.

---

## Phase 3.6 — Multi-agent Supervisor & Guardrails

**Goal**: route each turn to the right specialist agent (tools vs. web) instead of always offering every tenant tool at once, and let a business declare an external-facing bot with content-disclosure rules that are actually enforced, not just prompted-and-hoped-for.

- [x] `BotProfile.visibility` ("internal" | "external") + `BotProfile.guardrails` (free-text disclosure rules), validated in `CreateBotProfile`
- [x] `orchestrator/server/router.py` — LLM classification into `tools` (the tenant's own registered connectors/HttpTools) or `web` (public search); `analytics` is an explicit, documented alias for `tools`, not a fabricated third capability — see `docs/architecture/ARCHITECTURE.md` §3
- [x] `orchestrator/server/web_search.py` — built-in web-search tool (DuckDuckGo HTML endpoint, no API key), offered only on the `web` route
- [x] `orchestrator/server/guardrails.py` — LLM-as-judge screening, two checkpoints: a tool's raw result before it enters context, and the final answer before it's sent
- [x] `chat_service.py`: guardrail-active turns buffer the full answer (real streaming and hard guardrails are mutually exclusive — see `docs/architecture/SECURITY.md` §7) and send it chunked after passing the screen; non-guardrail turns keep genuine token streaming, unchanged

**Definition of done**: ✅ **met.** Registered a tool whose real API response bundles order status with a supplier name, created an `external` profile with the guardrail "never disclose supplier names," and asked for both status and supplier in one message. Inspected the actual message the model received (not just its final answer) and confirmed the tool result was redacted to `[content withheld...]` *before* generation — the model never saw the supplier name, so it couldn't have disclosed it regardless of phrasing. Confirmed the guardrail judge genuinely discriminates (a control call with safe text passes, the same text with the supplier name fails) — the redaction isn't a blanket "always fail closed." Separately verified web routing: a general-knowledge question logged `route=web`, made a real `POST` to `https://html.duckduckgo.com/html/`, and answered correctly from real search results.

**Notes**:
- **A real, honest limitation found in live verification**: guardrail screening operates on the whole tool-result text, not per-field. Because the fake verification API returns status/eta/supplier as one JSON blob, *any* call to that tool gets redacted once a guardrail forbids supplier disclosure — even a query that only asked about shipping status, nothing to do with the supplier. Safe (fails toward less disclosure), but blunt. Field-level redaction is a real follow-up (either LLM-directed partial redaction, or marking specific schema fields sensitive so `mcp-gateway` strips them before returning), not built here — documented in `docs/architecture/SECURITY.md` §7 rather than silently left as a surprise.
- **Streaming/guardrail tension was thought through before writing code, not discovered after**: a hard guardrail that actually prevents disclosure cannot coexist with real-time token streaming, since a token already sent can't be recalled. Buffering the full answer for guardrail-active turns only (not every turn) is the deliberate resolution — see `SECURITY.md` §7's explicit note not to "fix" this by trying to stream guardrail-checked content incrementally.
- **"Analytics" is a routing category, not a capability** — there's no separate analytics data source (metrics store, BI connector type) in this system yet. Routing still classifies it distinctly so the alias is visible and can be swapped for a real backend later by changing one mapping, rather than pretending a capability exists that doesn't.
- `orchestrator/server/web_search.py` uses regex extraction against DuckDuckGo's HTML-lite endpoint rather than a full HTML parser (avoids adding `beautifulsoup4` for what's currently one page's worth of scraping) — degrades to "no results" if DuckDuckGo's markup changes, not a crash, but is fragile by nature; swap for a real search API if reliability matters more than avoiding a paid dependency.
- 22 new tests across `test_guardrails.py`, `test_router.py`, `test_web_search.py`, plus new cases in `test_graph.py` and `test_assembly.py` — 49 orchestrator tests total, all passing.

---

## Phase 4+ — not yet planned in detail

RAG, memory, multi-agent planning, `compute`, real channels (`web-widget`, WhatsApp, Slack), the `web` onboarding dashboard, observability (Langfuse/OTel), and deployment (Kubernetes) all follow the same shape as the architecture doc describes, but aren't broken into phases yet — do that once Phase 3's dynamic-tool-assembly milestone is real and proven, not before.

## Parking lot
- Connector template marketplace, no-code onboarding, vertical `packs/` marketplace
- Usage-based billing, white-label tiers
- Multi-language support
- Kubernetes deployment
