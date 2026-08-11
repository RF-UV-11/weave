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

## Phase 3 — Orchestrator Core + MCP Client

**Goal**: `orchestrator` exists, streams a real LLM response, and can call a real MCP connector via dynamic tool assembly.

- [ ] `protos/orchestrator/v1/chat.proto` — server-streaming `ChatStream`; wire Python codegen
- [ ] `orchestrator/` Python project: `server/chat_service.py`, `llm/` (Ollama first), `mcp_client/` (initialize → tools/list → tools/call)
- [ ] `orchestrator/tools/assembly.py` — the dynamic tool-assembly mechanism from `docs/architecture/ARCHITECTURE.md` §3
- [ ] `packages/shared-clients` (Python) — generated gRPC client to `core`
- [ ] Reference connector: `connectors/reference-mcp/` — a trivial MCP server (e.g. a fake booking tool) to develop against
- [ ] Minimal dev UI (chat test harness — CLI or a bare Streamlit page, not the real `web` app) to watch it work

**Definition of done**: ask the dev harness a question that requires the reference connector's tool; see the tool discovered dynamically (not hardcoded), called, and the answer streamed back — end-to-end through `core` for tenant/profile resolution.
- [ ] **Tool description carried through to tool-call results** (requirement added post-Phase-1, see `docs/architecture/ARCHITECTURE.md` §3): when `tools/call` returns, the response handed to the planner/agent must include the tool's description alongside the raw result — not the raw result alone. `core` already enforces descriptions are present at registration (`RefreshManifest` rejects any tool missing one); this item is enforcing the other half, that the description isn't dropped after that point.

**Notes**:

---

## Phase 4+ — not yet planned in detail

RAG, memory, multi-agent planning, `compute`, real channels (`web-widget`, WhatsApp, Slack), the `web` onboarding dashboard, observability (Langfuse/OTel), and deployment (Kubernetes) all follow the same shape as the architecture doc describes, but aren't broken into phases yet — do that once Phase 3's dynamic-tool-assembly milestone is real and proven, not before.

## Parking lot
- Connector template marketplace, no-code onboarding, vertical `packs/` marketplace
- Usage-based billing, white-label tiers
- Multi-language support
- Kubernetes deployment
