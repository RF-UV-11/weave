# Build Plan — ServiceSphere AI

How to use this file with Claude Code:
- Work top to bottom. Don't start a phase until the previous phase's "Definition of done" is fully checked.
- At the start of a session, say something like: *"Read CLAUDE.md and PLAN.md, then continue from the next unchecked task."*
- After finishing a task, tick its box `[x]` and add a one-line note under that phase's **Notes** if you hit a real gotcha or made a decision — future sessions read this instead of re-discovering it.
- **Before checking a phase's "Definition of done," generate/update that phase's topic docs per `LEARNING.md`** — each phase maps to one module there, with an exact file path and required template per topic.
- Full rationale for *why* things are structured this way lives in `docs/architecture/OVERVIEW.md` — this file is the *what/when*, `LEARNING.md` is the *what to learn and where it's written up*, that file is the *why*.
- **The four spine rules** (from OVERVIEW.md): (1) services are grouped repos talking over gRPC/Connect; (2) `protos/` is the only contract source; (3) `backend-services` (Go) is the *only* tier that touches MongoDB; (4) `analysis-services` is one grouped compute server, capabilities are modules-via-routes.
- Track cumulative implementation state and releases in `CHECKLIST.md`.
- **Build order is module-by-module, not vertical-slice.** Each phase finishes one service group (or a coherent chunk of one) before the next starts. `frontend-services/web` (the Next.js app) is deliberately the *last* build module (Phase 12) — everything before it is verified via `grpcurl`/`buf curl`/scripts, with the Streamlit demo (Phase 2) as the one lightweight dev-visualization tool along the way.

---

## Phase 0 — Bootstrap, Contracts & Data Tier

**Goal**: the skeleton plus the *spine*: a real `protos/` contract, generated stubs, and a Go `backend-services` RPC writing to MongoDB. Nothing touches the DB except this tier, from day one.

- [x] Init git repo, `.gitignore` (Go, Python, Node, `gen/`, `.env`)
- [x] Create top-level groups per `CLAUDE.md` repo layout: `protos/`, `backend-services/`, `ai-services/`, `analysis-services/`, `frontend-services/`, `channels/`, `mcp-servers/`, `domain-packs/`, `packages/`, `infra/`, `docs/`, `scripts/`
- [x] `protos/buf.yaml` + `protos/buf.gen.yaml` — codegen to Go (`backend-services/gen`); Python targets added in Phase 2 once `ai-services` exists; `buf lint` clean
- [x] First contract: `protos/backend_services/data_access/v1/ticket.proto` (`CreateTicket`, `GetTicket`, `ListTickets`) + `protos/database/v1/` shared types (`PageRequest`/`PageResponse`, `ErrorDetail`, `TenantScope`, `Money`, `Health`)
- [x] `backend-services/` Go module, RPC-per-collection layout (see `backend-services/CLAUDE.md`): `mongodb/` (initialize.go, collections.go, indexes.go, health.go, ticket.go — the only Mongo access), `rpc_services/ticket/` (server.go + routehandler.go), `configs/`, `main.go` at module root (Connect server + `Health` RPC) — `go build ./...` and `go vet ./...` pass clean
- [x] `infra/podman-compose.yml` with `mongo`, `redis:7`, `qdrant/qdrant`, `minio/minio`, healthchecks on all, plus `backend-services`; `infra/containerfiles/backend-services.Containerfile`
- [x] `.env.example` at root listing every env var the stack will need (Mongo URI, Redis URL, Qdrant URL, MinIO creds, JWT secret, LLM provider keys) — even ones not used yet
- [x] Root `Makefile` wrapping `CLAUDE.md` commands (`make up`, `make gen`, `make down`, `make logs`, `make lint`, `make build-backend`, `make test-backend`)
- [ ] Confirm `podman-compose -f infra/podman-compose.yml up -d` brings up all 4 infra containers healthy, and `CreateTicket` over Connect writes a real Mongo document

**Definition of done**: `podman-compose up -d` gives healthy Mongo/Redis/Qdrant/MinIO, and a `grpcurl`/Connect call to `backend-services` `CreateTicket` inserts a ticket into MongoDB — no Python and no frontend yet.

**Notes**:
- Dev environment (Windows) had no git/go/buf/podman preinstalled — installed via `winget` (Git.Git, GoLang.Go, RedHat.Podman) and `go install` for `buf`/`protoc-gen-go`/`protoc-gen-connect-go`. After any winget install, PATH must be re-read from the registry in a *new* PowerShell process (`[Environment]::GetEnvironmentVariable("Path","Machine")` + `"...User"`) — it does not propagate to already-open shells.
- Podman on Windows requires a WSL2-backed machine (`podman machine init`). WSL2 itself needs the "Virtual Machine Platform" Windows feature + BIOS virtualization, enabled via an **elevated** `wsl.exe --install --no-distribution` + reboot — this could not be done from a non-interactive/unelevated session. **`podman-compose up` is unverified pending that.**
- `buf`'s STANDARD lint rules require RPC request/response message names to match the RPC method (`Check`/`CheckRequest`/`CheckResponse`, not `HealthCheckRequest`/`HealthCheckResponse`) — worth remembering before writing the next service's protos.
- Mongo driver: used `go.mongodb.org/mongo-driver/v2` (not v1) — `mongo.Connect(opts)` takes no context arg, index options come from the `mongo/options` package (`options.Index()`, not `mongo.IndexOptions()`).
- `backend-services`' internal layout follows an RPC-per-collection convention (`mongodb/<collection>.go` + `rpc_services/<collection>/{server,routehandler}.go`, `main.go` at module root) adapted from a reference architecture the user pointed at — full rationale in `backend-services/CLAUDE.md`. Every `is_collection: true` entity's first field is `_id` (not `id`) so Mongo's own primary key is the ID — the Go field becomes `XId`/`GetXId()` (protoc-gen-go can't emit `Id` for a leading-underscore field); this is expected, silence the linter with `// buf:lint:ignore FIELD_LOWER_SNAKE_CASE`. See the `_id` convention note in root `CLAUDE.md`.
- Fetching a private reference repo mid-session: installed `gh` CLI but a PAT pasted directly in chat was used for one-off `Invoke-RestMethod` calls instead (never `gh auth login`'d, never written to a file) — the local copy of fetched files was deleted immediately after extracting the pattern, since it was another company's proprietary source. If this comes up again, prefer the user running `gh auth login` themselves so no token touches the transcript.
- **2026-07-23: re-planned into module-by-module build order** (this file's phase list below), with `frontend-services/web` deliberately last (Phase 12). Every old phase's content was preserved and reassigned to a new phase number — see `CHECKLIST.md` §3 for the phase→version mapping and `LEARNING.md` for the matching module→phase renumbering.

---

## Phase 1 — Backend Core Domains

**Goal**: round out `backend-services` with the collections the earliest `ai-services` work needs — chat/session storage, knowledge base, calendar, and auth — before any Python orchestration code exists. Same RPC-per-collection pattern as `ticket` (Phase 0); test every RPC with `grpcurl`/`buf curl`, no client code needed yet.

- [ ] **chat** domain — `protos/database/v1/chat.proto` (`ChatSession`, `ChatMessage`, `UserMemory`, `UserPreference`, all `is_collection: true`) + `protos/backend_services/data_access/v1/chat.proto` (`AppendMessage`, `GetSession`, `WriteMemory`, `GetPreferences`); `mongodb/chat.go`; `rpc_services/chat/`
- [ ] **knowledge-base** domain — `protos/database/v1/knowledge.proto` (`KbArticle`) + RPCs `ListArticles`, `UpsertArticle`; `mongodb/knowledge.go`; `rpc_services/knowledge/`
- [ ] **calendar** domain — `protos/database/v1/calendar.proto` (`Meeting`, `AvailabilitySlot`) + RPCs `GetAvailability`, `BookMeeting`; `mongodb/calendar.go`; `rpc_services/calendar/`
- [ ] **auth** domain — `protos/database/v1/auth.proto` (`User`, `Role`) + RPCs `Register`, `Login`, `Refresh`, `AssignRole`, JWT issuing (15min access / 7day refresh, rotation); `mongodb/auth.go`; `rpc_services/auth/`
- [ ] `packages/shared-auth` (Go side) — JWT-verify Connect interceptor + `requires_role(...)` helper; not yet wired into every RPC (that's Phase 10) but available and unit-tested
- [ ] Register all four new services in `backend-services/main.go`; add index blocks to `mongodb/indexes.go` for each
- [ ] Verify each RPC with `grpcurl`/`buf curl` against the running `podman-compose` stack

**Definition of done**: `backend-services` exposes working `ticket`, `chat`, `knowledge-base`, `calendar`, and `auth` RPCs, each verified against real MongoDB via `grpcurl`/`buf curl` — still zero Python, zero frontend.

**Notes**:

---

## Phase 2 — AI Services Core

**Goal**: stand up `ai-services`, get a real streaming chat RPC talking to an LLM, add the first tool call, and get a lightweight visual way (Streamlit) to watch it work — before building RAG/memory/multi-agent on top.

- [ ] `protos/ai_services/v1/chat.proto` — `rpc ChatStream(ChatRequest) returns (stream ChatChunk)` (server-streaming); wire Python codegen into `protos/buf.gen.yaml` (`ai-services/gen`)
- [ ] `ai-services/` Python project: `server/chat_service.py` (Connect server via `connecpy`, `Health` RPC), `pyproject.toml`, `Containerfile`
- [ ] `ai-services/llm/` — thin provider abstraction, one `stream_completion(messages) -> AsyncIterator[str]`, implemented against Ollama's OpenAI-compatible endpoint first (pull `llama3.2` or `qwen2.5`)
- [ ] `ChatStream` streams token chunks; basic "ServiceSphere AI assistant" system prompt; no persistence yet (Phase 5 adds that)
- [ ] `packages/shared-clients` (Python side) — pre-wired Connect client from `ai-services` to `backend-services`
- [ ] `ai-services/tools/ticket_tools.py` — `create_ticket(input: CreateTicketInput) -> CreateTicketOutput`, Pydantic-typed, calling the generated Connect client to `backend-services` `CreateTicket` (no DB access in `ai-services`)
- [ ] Wire the tool into the LLM call using real function-calling (not prompt-hacking) — model decides to call `create_ticket`, `ai-services` executes it via gRPC to Go, result goes back to the model, model responds in natural language
- [ ] **Streamlit demo** (`frontend-services/streamlit-demo/`) as a **dev tool**, not the final UI: `app.py` with `st.chat_message` + `st.write_stream` against the `ChatStream` RPC, minimal theming — good enough to watch tool calls / RAG / memory / multi-agent as they get built in later phases
- [ ] Wire `podman-compose.yml` to also run `ai-services` and the Streamlit demo for dev

**Definition of done**: open the Streamlit demo, type a message, see a real streamed LLM response; ask "I'm having a login problem, can you open a ticket?" and get a real Mongo document (written by `backend-services`) plus a natural-language confirmation — `ai-services` holding zero DB credentials throughout.

**Notes**:

---

## Phase 3 — Channels

**Goal**: prove the chat RPC is channel-agnostic by building the thin adapters early, right after core chat + one tool exist. Full multi-tenant routing isn't ready yet (Phase 11) — these run against the single default tenant (`DEFAULT_TENANT_ID` from `.env.example`) for now.

- [ ] **Embeddable web widget** (`channels/web-widget/`) — small vanilla-JS/preact bundle, one `<script src=… data-tenant=…>` tag, opens a chat bubble, calls the chat RPC (gRPC-Web/JSON) with a scoped public API key, CORS locked to a configured origin
- [ ] **WhatsApp adapter** (`channels/whatsapp/`) — webhook verifying Meta's challenge token, buffers the streamed response into one outbound message (or progressive edits if the provider supports it)
- [ ] **Slack adapter** (`channels/slack/`, optional/stretch) — slash command or app-mention triggers the same chat RPC
- [ ] End-to-end test: same question via the widget (local test page) and a sandbox WhatsApp number produces consistent tool use

**Definition of done**: the widget and WhatsApp adapter both round-trip a real message through `ai-services`' chat RPC and get a coherent response, without either adapter containing any business logic.

**Notes**:

---

## Phase 4 — RAG

**Goal**: chat answers grounded in real documents with citations, visible in the Streamlit demo.

- [ ] Seed ~15–20 real FAQ/policy articles into the `knowledge-base` domain (Phase 1) via its `UpsertArticle` RPC
- [ ] `ai-services/rag/chunking.py` — hybrid fixed-size + heading-aware splitter, ~15% overlap
- [ ] `ai-services/rag/embed.py` — embedding call (local/free via Ollama or `sentence-transformers`), write vectors + metadata to Qdrant
- [ ] Ingestion script `scripts/ingest_kb.py` — pulls all articles **via the `backend-services` RPC** (not Mongo), chunks, embeds, upserts to Qdrant with metadata (`tenant_id`, `source_id`, `category`, `visibility`)
- [ ] `ai-services/rag/retrieve.py` — hybrid search (Qdrant dense + sparse) → top ~20 → rerank (cross-encoder) → top 4–8
- [ ] `ai-services/agents/rag_agent.py` — retrieval pipeline feeding context + citation instructions into generation
- [ ] Streamlit demo: render citation markers below the assistant's answer

**Definition of done**: ask a question only answerable from a seeded KB article in the Streamlit demo, get a correct answer with a visible citation, where the source rows came from `backend-services`.

**Notes**:

---

## Phase 5 — Memory

**Goal**: the assistant remembers across turns and sessions, using the `chat` domain built in Phase 1.

- [ ] Session persisted via `AppendMessage` (Phase 1's `chat` domain) on every turn
- [ ] Redis-backed short-term cache of the last N messages for the active session (fast path, avoids an RPC round-trip every turn)
- [ ] `ai-services/memory/memory_agent.py` — after each turn, a lightweight structured-output call: "worth remembering long-term?" → writes via `WriteMemory` RPC + embeds into Qdrant
- [ ] On a new turn: load last-K short-term (Redis) + top-N semantically relevant long-term (Qdrant) + preferences (`GetPreferences` RPC), assembled into system context before generation

**Definition of done**: tell the Streamlit demo your name/preference in one session, start a new session later, and have it recall that fact unprompted when relevant.

**Notes**:

---

## Phase 6 — Multi-Agent (LangGraph)

**Goal**: a real Planner routing between multiple specialist agents.

- [ ] `ai-services/graph/state.py` — shared `GraphState` Pydantic model (messages, session context, planner decision, collected tool results)
- [ ] `ai-services/graph/planner.py` — Planner node with structured output `PlannerDecision{agents_needed, requires_rag, requires_clarification}`
- [ ] `ai-services/graph/build_graph.py` — assemble `StateGraph` with conditional edges routing to Sales / Support / Finance / RAG agents; reasoning/merge node combines outputs
- [ ] Migrate the ticket tool (Phase 2) and RAG agent (Phase 4) into the graph as proper nodes
- [ ] Test a compound query needing two agents in one turn ("what's my ticket status, and can you also book a follow-up call") — both fire and merge into one reply (calendar agent lands properly in Phase 8/MCP; a direct-RPC stand-in is fine here)
- [ ] **Comparison exercise**: reimplement the Sales Agent alone in Pydantic AI; write up the tradeoffs in `docs/roadmap/phase-6.md`

**Definition of done**: the compound query produces one merged, correct answer in the Streamlit demo, and the Langfuse trace (once Phase 9 lands) will show the Planner's routing decision — for now, log it clearly to stdout/trace stub.

**Notes**:

---

## Phase 7 — Grouped Compute (`analysis-services`)

**Goal**: stand up the grouped compute tier — one server, capabilities as modules-via-routes.

- [ ] `protos/analysis_services/v1/` — `estimation.proto` (`EstimateProject`), `analytics.proto` (`GetAnalytics`)
- [ ] `analysis-services/server/app.py` — the ONE common Connect server; registers every module's route + `Health`
- [ ] `analysis-services/estimation/` — `estimateProject` math as a module (pure functions; pulls any needed data via `backend-services` RPC, never Mongo)
- [ ] `analysis-services/analytics/` — dashboard aggregations as a module
- [ ] `ai-services/tools/estimate_tools.py` + `analytics_tools.py` — tools resolving to `analysis-services` routes; register with the Planner graph
- [ ] Prove the pattern: add a second capability (e.g. `scoring`) as a module with **no new deployment** — only a route on the same server

**Definition of done**: `estimateProject` and `getAnalytics` both run as modules behind one `analysis-services` server, reachable via routes, with data (where needed) fetched from `backend-services`, and callable from the Streamlit demo through a tool.

**Notes**:

---

## Phase 8 — MCP

**Goal**: build one real MCP server and one real MCP client, end to end, using the `calendar` domain built in Phase 1.

- [ ] `mcp-servers/calendar-mcp/server.py` — `initialize`, resource `calendar://availability`, tool `book_meeting`, prompt `suggest_times`; transport stdio for local dev
- [ ] `ai-services/mcp_clients/calendar_client.py` — `initialize` → `tools/list` → `tools/call`, results returned as structured content
- [ ] `ai-services/agents/calendar_agent.py` uses the MCP client instead of a plain tool, and replaces Phase 6's direct-RPC calendar stand-in in the graph
- [ ] Verify the same `calendar-mcp` server works against a generic MCP client (Claude Desktop / MCP inspector) to prove it's protocol-standard

**Definition of done**: book a meeting through the Streamlit demo, confirm the flow goes `initialize → tools/list → tools/call` against the MCP server (which itself calls `backend-services` for data, never Mongo directly).

**Notes**:

---

## Phase 9 — Observability & Evaluation

**Goal**: you can see what the system is doing and catch regressions automatically.

- [ ] Stand up Langfuse (self-hosted via `podman-compose` or cloud); wire `ai-services` LLM/tool/RPC calls to emit traces tagged with `tenant_id`/`user_id`/`session_id`/`agent_name`
- [ ] OpenTelemetry collector config in `infra/observability/`; instrument all Go + Python services (gRPC/Connect interceptors) for RPC latency/error rate → Prometheus, basic Grafana dashboard
- [ ] `ai-services/evals/` — golden dataset of (query → expected tool call / expected answer) pairs covering each agent
- [ ] `ai-services/agents/evaluation_agent.py` — post-hoc faithfulness/hallucination check on RAG answers (scoring can run in `analysis-services/scoring`)
- [ ] Wire the eval harness into CI (`.github/workflows/eval.yml`) so a PR touching `ai-services` runs the golden set and reports pass rate
- [ ] Define and wire one real alert (RAG faithfulness below threshold, or tool-call/RPC error rate spike)

**Definition of done**: open Langfuse and follow one conversation from the Planner's decision through every tool call and cross-service RPC to the final token, with cost and latency at each step; CI fails on a golden-eval regression.

**Notes**:

---

## Phase 10 — Backend Completion

**Goal**: round out `backend-services` with the remaining domains needed for the full IT-services reference vision, now that `ai-services` exists to actually use them.

- [ ] Build out remaining `backend-services` domains: `customer`, `crm`, `project`, `invoice`, `proposal`, `notification`, `analytics`, `document` — each: entity proto (`is_collection: true`, `_id` first field), RPC contract, `mongodb/<name>.go`, `rpc_services/<name>/`, index block, registered in `main.go`
- [ ] Add corresponding tools into `ai-services/tools/` (`getInvoices`, `generateProposal`, `generateInvoice`, `uploadDocument`, `summarizeDocument`, `sendEmail`, `trackProject`) and register with the relevant agent in the Planner graph
- [ ] Wire `packages/shared-auth`'s interceptor (built in Phase 1) into every `backend-services` RPC now, not just `auth`'s own
- [ ] Re-verify RBAC at the tool layer AND at the `backend-services` RPC for sensitive ops (`generateInvoice`/`generateProposal` require `finance`/`admin`/`sales_rep` as appropriate)
- [ ] `idempotency_key` handling on invoice/payment-creating RPCs (dedupe in `backend-services`)

**Definition of done**: a logged-in customer and a logged-in admin get genuinely different tool access in the same Streamlit demo session, enforced at both the tool layer and the Go data tier.

**Notes**:

---

## Phase 11 — Domain Packs & Multi-Tenancy

**Goal**: prove the core is actually generic (domain packs) and safe for multiple firms (multi-tenancy) — do this only after the reference implementation works end-to-end.

- [ ] Create `domain-packs/_template/` with `business.yaml`, `branding.yaml`, `system_prompt.md`, `knowledge/` per OVERVIEW.md §21
- [ ] Move all IT-services-specific content (the 14 services list, system prompt persona, seeded KB articles) out of hardcoded core files into `domain-packs/it-services/`
- [ ] `ai-services`: add a domain-pack loader — reads `tenant_id`, loads/caches the matching pack, uses `agents_enabled`/`tools_enabled` to filter what the Planner is offered
- [ ] Grep the whole `backend-services`/`ai-services`/`analysis-services` tree for remaining hardcoded IT-services strings/logic; move each into config
- [ ] Prove genericity: build a **second** domain pack for a distinct vertical (clinic / e-commerce) with its own config + a few seeded KB docs — no core code changes allowed
- [ ] `scripts/new-firm.sh` — scaffolds `_template/` into a new slug, prompts for required fields, runs KB ingestion
- [ ] `tenant_id` field + index on every tenant-scoped Mongo collection; every `backend-services` RPC requires and enforces `tenant_id`
- [ ] Qdrant: one collection per tenant (start simple); ingestion and retrieval always scope by `tenant_id`
- [ ] Extend RBAC checks (`packages/shared-auth`) to be tenant-scoped: role X *within tenant Y*
- [ ] Properly wire `channel_tenant_map` (Phase 3's channels currently run against a single default tenant) — lookup collection mapping WhatsApp phone-number-ID / widget API key to `tenant_id`, populated by `scripts/new-firm.sh`
- [ ] Per-tenant rate limiting and basic cost/usage tracking (token counts from Langfuse tagged by tenant) at the edge
- [ ] Isolation test: seed two tenants with overlapping data (both have a ticket "Login broken"), confirm a query scoped to tenant A never returns tenant B's document or RAG chunk

**Definition of done**: both domain packs run against the same core deployment, produce visibly different assistants, the diff between them touches only `domain-packs/`, the isolation test passes, and Langfuse traces are filterable by tenant.

**Notes**:

---

## Phase 12 — Frontend (Next.js Web App)

**Goal**: the last build module. Everything before this was verified via `grpcurl`/`buf curl`/the Streamlit demo — now build the real customer + admin portal.

- [ ] `frontend-services/web` Next.js skeleton: `create-next-app` (TS + Tailwind + shadcn init), Connect client generated from `protos/`
- [ ] `components/chat/ChatWindow.tsx` + `hooks/useChatStream.ts` consuming the server-streaming chat RPC, rendering tokens as they arrive
- [ ] Tool-call cards: "Creating ticket…" → "✅ Ticket #123 created", reused for every tool
- [ ] Citation markers as clickable source links (RAG, Phase 4)
- [ ] The Live Signal Path trace element per `DESIGN.md` §2 (Planner → Agent → Tool/MCP → Response), full animated version (Streamlit's was the simplified static approximation)
- [ ] Customer dashboard (projects, invoices, tickets) and Admin dashboard (CRM board, proposals, analytics, knowledge-base browser) — both wired to the domains built in Phases 1/10
- [ ] Auth flow (login/register) against Phase 1's `auth` domain; RBAC-aware UI (different nav/actions per role)
- [ ] Full `DESIGN.md` pass: dark-mode-first, token system, typography — confirm it reads as one coherent product alongside the Streamlit demo and widget
- [ ] Wire `podman-compose.yml` to run `web` for dev

**Definition of done**: open the web app, log in, chat with tool calls / citations / the full Live Signal Path trace rendering correctly, and navigate both the customer and admin dashboards — no more `grpcurl`-only verification needed for any feature.

**Notes**:

---

## Phase 13 — Deployment & OSS Release Readiness

**Goal**: the whole stack — including the frontend — runs on Kubernetes, and a stranger can find this repo, understand it, run it, and trust it enough to point it at their business.

- [ ] `Containerfile` per remaining service (multi-stage builds, non-root user); build with Podman/Buildah
- [ ] `podman generate kube` from the local pods as a first-pass manifest, then harden into `infra/k8s/base/` Kustomize bases (Deployment + Service + ConfigMap/Secret) per service
- [ ] `infra/k8s/overlays/dev/` — first overlay, deploy to local `kind`/`minikube`, confirm the full graph works cluster-internally (service DNS, not `localhost`)
- [ ] Ingress (Connect-aware) as cluster entrypoint, TLS termination
- [ ] HPA on `ai-services` and `backend-services` at minimum
- [ ] `.github/workflows/ci.yml` — `buf lint`/`breaking` → `go test`/`pytest` → build/push image; `deploy.yml` — `kubectl apply -k overlays/staging` on merge to main, manual promotion gate to `overlays/prod`
- [ ] `overlays/staging` and `overlays/prod` — same manifests, different secrets/limits/replicas (MongoDB/Redis managed in prod)
- [ ] `LICENSE` (MIT)
- [ ] Public root `README.md` rewritten for the OSS audience: elevator pitch, quickstart (`podman-compose up` → open Streamlit demo in under 10 minutes), supported channels, screenshot/GIF of the trace UI, links to `docs/architecture/OVERVIEW.md`, `PLAN.md`, `DESIGN.md`, license badge
- [ ] `CONTRIBUTING.md` — how to run tests, and the extension points (domain pack / channel / MCP server / data-access domain / analysis module), linking back to `CLAUDE.md`
- [ ] `CODE_OF_CONDUCT.md` (Contributor Covenant), `SECURITY.md` (private vulnerability reporting)
- [ ] `CHANGELOG.md` (Keep a Changelog format), tag `v0.1.0`; `CHECKLIST.md` version log kept current
- [ ] `.github/ISSUE_TEMPLATE/` — bug report, new-domain-pack request, new-channel request; `.github/PULL_REQUEST_TEMPLATE.md`
- [ ] Add `core_version` to `business.yaml` schema; `buf breaking` guarding contract compatibility in CI
- [ ] Full design-system pass: confirm Next.js app, Streamlit demo, and web widget all read as one visual product per `DESIGN.md`

**Definition of done**: a merged PR automatically builds, pushes, and rolls out to staging with zero manual steps; a first-time visitor can go from README to a running Streamlit demo without reading any other doc, and knows exactly where to look (`CONTRIBUTING.md`) to add their own domain pack or channel. Tag `v1.0.0`.

**Notes**:

---

## Parking lot (explicitly out of scope until the phases above are done)
- Multi-tenant billing / metered usage
- SSO / SAML for enterprise admin accounts
- Fine-tuning or distillation of a smaller in-house model
- Non-English language support
- Promoting an `analysis-services` module to its own scalable service (only if profiling demands it)

Add things here instead of scope-creeping into an active phase.
