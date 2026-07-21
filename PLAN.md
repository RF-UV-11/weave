# Build Plan — ServiceSphere AI

How to use this file with Claude Code:
- Work top to bottom. Don't start a phase until the previous phase's "Definition of done" is fully checked.
- At the start of a session, say something like: *"Read CLAUDE.md and PLAN.md, then continue from the next unchecked task."*
- After finishing a task, tick its box `[x]` and add a one-line note under that phase's **Notes** if you hit a real gotcha or made a decision — future sessions read this instead of re-discovering it.
- **Before checking a phase's "Definition of done," generate/update that phase's topic docs per `LEARNING.md`** — each phase maps to one module there, with an exact file path and required template per topic.
- Full rationale for *why* things are structured this way lives in `docs/architecture/OVERVIEW.md` — this file is the *what/when*, `LEARNING.md` is the *what to learn and where it's written up*, that file is the *why*.
- **The four spine rules** (from OVERVIEW.md): (1) services are grouped repos talking over gRPC/Connect; (2) `protos/` is the only contract source; (3) `backend-services` (Go) is the *only* tier that touches MongoDB; (4) `analysis-services` is one grouped compute server, capabilities are modules-via-routes.
- Track cumulative implementation state and releases in `CHECKLIST.md`.

---

## Phase 0 — Bootstrap, Contracts & Data Tier

**Goal**: the skeleton plus the *spine*: a real `protos/` contract, generated stubs, and a Go `backend-services` RPC writing to MongoDB. Nothing touches the DB except this tier, from day one.

- [x] Init git repo, `.gitignore` (Go, Python, Node, `gen/`, `.env`)
- [x] Create top-level groups per `CLAUDE.md` repo layout: `protos/`, `backend-services/`, `ai-services/`, `analysis-services/`, `frontend-services/`, `channels/`, `mcp-servers/`, `domain-packs/`, `packages/`, `infra/`, `docs/`, `scripts/`
- [x] `protos/buf.yaml` + `protos/buf.gen.yaml` — codegen to Go (`backend-services/gen`); Python targets added in Phase 1 once `ai-services` exists; `buf lint` clean
- [x] First contract: `protos/backend_services/data_access/v1/ticket.proto` (`CreateTicket`, `GetTicket`, `ListTickets`) + `protos/database/v1/` shared types (`PageRequest`/`PageResponse`, `ErrorDetail`, `TenantScope`, `Money`, `Health`)
- [x] `backend-services/` Go module: `database/mongo.go` (the ONLY Mongo client), `database/repositories/ticket_repo.go`, `database/migrate/` (ensure indexes), `data-access/ticket/` handler, `data-access/cmd/server/main.go` (Connect server + `Health` RPC) — `go build ./...` and `go vet ./...` pass clean
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

---

## Phase 1 — Basic Chat (no tools yet)

**Goal**: one streaming chat RPC, end to end, talking to a local Ollama model. Proves the wiring before any complexity.

- [ ] `protos/ai_services/v1/chat.proto` — `rpc ChatStream(ChatRequest) returns (stream ChatChunk)` (server-streaming)
- [ ] `ai-services/` Python project: `server/chat_service.py` (Connect server via `connecpy`, `Health` RPC), `pyproject.toml`, `Containerfile`
- [ ] `ai-services/llm/` — thin provider abstraction, one `stream_completion(messages) -> AsyncIterator[str]`, implemented against Ollama's OpenAI-compatible endpoint first (pull `llama3.2` or `qwen2.5`)
- [ ] `ChatStream` streams token chunks; basic "ServiceSphere AI assistant" system prompt; no persistence, no tools yet
- [ ] `frontend-services/web` Next.js skeleton: `create-next-app` (TS + Tailwind + shadcn init), Connect client generated from `protos/`
- [ ] `components/chat/ChatWindow.tsx` + `hooks/useChatStream.ts` consuming the server-streaming chat RPC, rendering tokens as they arrive
- [ ] Wire `podman-compose.yml` to also run `ai-services` and (optionally) `web` for dev

**Definition of done**: open the web app, type a message, see a real streamed LLM response over Connect with no page reload, no tools, no auth.

**Notes**:

---

## Phase 2 — Tool Calling

**Goal**: the model takes one real action — a tool that calls `backend-services`, which is the only tier touching Mongo.

- [ ] `ai-services/tools/ticket_tools.py` — `create_ticket(input: CreateTicketInput) -> CreateTicketOutput`, Pydantic-typed, calling the generated Connect client to `backend-services` `CreateTicket` (no DB access in `ai-services`)
- [ ] Wire the tool into the LLM call using real function-calling (not prompt-hacking) — model decides to call `create_ticket`, `ai-services` executes it via gRPC to Go, result goes back to the model, model responds in natural language
- [ ] Render a "Creating ticket…" → "✅ Ticket #123 created" card in the chat UI when a tool call happens (the pattern reused for every future tool)
- [ ] Add `packages/shared-clients` — pre-wired Connect clients from `ai-services` to `backend-services`

**Definition of done**: ask "I'm having a login problem, can you open a ticket?" and get a real Mongo document (written by `backend-services`) plus a natural-language confirmation — with `ai-services` holding zero DB credentials.

**Notes**:

---

## Phase 3 — RAG

**Goal**: chat answers grounded in real documents with citations.

- [ ] `protos/backend_services/data_access/v1/knowledge.proto` + `backend-services` `knowledge-base` domain (`kb_articles` collection), seed ~15–20 real FAQ/policy articles about the 14 services
- [ ] `ai-services/rag/chunking.py` — hybrid fixed-size + heading-aware splitter, ~15% overlap
- [ ] `ai-services/rag/embed.py` — embedding call (local/free via Ollama or `sentence-transformers`), write vectors + metadata to Qdrant
- [ ] Ingestion script `scripts/ingest_kb.py` — pulls all articles **via the `backend-services` RPC** (not Mongo), chunks, embeds, upserts to Qdrant with metadata (`tenant_id`, `source_id`, `category`, `visibility`)
- [ ] `ai-services/rag/retrieve.py` — hybrid search (Qdrant dense + sparse) → top ~20 → rerank (cross-encoder) → top 4–8
- [ ] `ai-services/agents/rag_agent.py` — retrieval pipeline feeding context + citation instructions into generation
- [ ] Frontend: render citation markers as clickable source links

**Definition of done**: ask a question only answerable from a seeded KB article, get a correct answer with a visible citation, where the source rows came from `backend-services`.

**Notes**:

---

## Phase 4 — Memory

**Goal**: the assistant remembers across turns and sessions.

- [ ] `protos/backend_services/data_access/v1/chat.proto` + `backend-services` `chat` domain (`chat_sessions`, `chat_messages`, `user_memories`, `user_preferences` collections); session persisted via `AppendMessage` on every turn
- [ ] Redis-backed short-term cache of the last N messages for the active session (fast path, avoids an RPC round-trip every turn)
- [ ] `ai-services/memory/memory_agent.py` — after each turn, a lightweight structured-output call: "worth remembering long-term?" → writes via `WriteMemory` RPC + embeds into Qdrant
- [ ] On a new turn: load last-K short-term (Redis) + top-N semantically relevant long-term (Qdrant) + preferences (`GetPreferences` RPC), assembled into system context before the Planner runs

**Definition of done**: tell the assistant your name/preference in one session, start a new session later, and have it recall that fact unprompted when relevant.

**Notes**:

---

## Phase 5 — MCP

**Goal**: build one real MCP server and one real MCP client, end to end.

- [ ] Pick **Calendar MCP** as the first server (good complexity: resource, tool, prompt)
- [ ] `mcp-servers/calendar-mcp/server.py` — `initialize`, resource `calendar://availability`, tool `book_meeting`, prompt `suggest_times`; transport stdio for local dev
- [ ] `protos/backend_services/data_access/v1/calendar.proto` + `backend-services` `calendar` domain (`meetings`, `availability_slots`) — the MCP server calls this RPC, it never owns data
- [ ] `ai-services/mcp_clients/calendar_client.py` — `initialize` → `tools/list` → `tools/call`, results returned as structured content
- [ ] `ai-services/agents/calendar_agent.py` uses the MCP client instead of a plain tool — compare against Phase 2's direct-RPC tool to see the difference explicitly
- [ ] Verify the same `calendar-mcp` server works against a generic MCP client (Claude Desktop / MCP inspector) to prove it's protocol-standard

**Definition of done**: book a meeting through chat, watch the Langfuse trace show `initialize → tools/list → tools/call` against the MCP server (which itself calls `backend-services` for data).

**Notes**:

---

## Phase 6 — Multi-Agent (LangGraph)

**Goal**: a real Planner routing between multiple specialist agents.

- [ ] `ai-services/graph/state.py` — shared `GraphState` Pydantic model (messages, session context, planner decision, collected tool results)
- [ ] `ai-services/graph/planner.py` — Planner node with structured output `PlannerDecision{agents_needed, requires_rag, requires_clarification}`
- [ ] `ai-services/graph/build_graph.py` — assemble `StateGraph` with conditional edges routing to Sales / Support / Finance / RAG agents; reasoning/merge node combines outputs
- [ ] Migrate the ticket tool (P2), RAG agent (P3), and calendar agent (P5) into the graph as proper nodes
- [ ] Test a compound query needing two agents in one turn ("what's my ticket status, and can you also book a follow-up call") — both fire and merge into one reply
- [ ] **Comparison exercise**: reimplement the Sales Agent alone in Pydantic AI; write up the tradeoffs in `docs/roadmap/phase-6.md`

**Definition of done**: the compound query produces one merged, correct answer, and the Langfuse trace shows the Planner's routing decision.

**Notes**:

---

## Phase 7 — Grouped Compute (`analysis-services`)

**Goal**: stand up the grouped compute tier — one server, capabilities as modules-via-routes.

- [ ] `protos/analysis_services/v1/` — `estimation.proto` (`EstimateProject`), `analytics.proto` (`GetAnalytics`)
- [ ] `analysis-services/server/app.py` — the ONE common Connect server; registers every module's route + `Health`
- [ ] `analysis-services/estimation/` — `estimateProject` math as a module (pure functions; pulls any needed data via `backend-services` RPC, never Mongo)
- [ ] `analysis-services/analytics/` — dashboard aggregations as a module
- [ ] `ai-services/tools/estimate_tools.py` + `analytics_tools.py` — tools resolving to `analysis-services` routes; register with Sales / (admin) agents
- [ ] Prove the pattern: add a second capability (e.g. `scoring`) as a module with **no new deployment** — only a route on the same server

**Definition of done**: `estimateProject` and `getAnalytics` both run as modules behind one `analysis-services` server, reachable via routes, with data (where needed) fetched from `backend-services`.

**Notes**:

---

## Phase 8 — Production Contracts & Auth (remaining data-access domains + auth)

**Goal**: the rest of the core data-access domains exist, and everything sits behind real auth.

- [ ] `backend-services` `auth` domain — `Register`/`Login`/`Refresh`, `users`/`roles`/`permissions` collections, JWT issuing (15min access / 7day refresh, rotation)
- [ ] `packages/shared-auth` — JWT-verify Connect interceptor + `requires_role(...)` helper (Go + Python), used by every service
- [ ] Build out remaining `backend-services` domains not yet built: `customer`, `crm`, `project`, `invoice`, `proposal`, `notification`, `analytics`, `document` — each: proto, repository, index bootstrap, RPCs, `Health`
- [ ] Add corresponding tools into `ai-services/tools/` (`getInvoices`, `generateProposal`, `generateInvoice`, `uploadDocument`, `summarizeDocument`, `sendEmail`, `trackProject`) and register with the relevant agent
- [ ] Re-verify RBAC at the tool layer AND at the `backend-services` RPC for sensitive ops (`generateInvoice`/`generateProposal` require `finance`/`admin`/`sales_rep` as appropriate)
- [ ] `idempotency_key` handling on invoice/payment-creating RPCs (dedupe in `backend-services`)

**Definition of done**: a logged-in customer and a logged-in admin get genuinely different tool access in the same chat, enforced at both the tool layer and the Go data tier.

**Notes**:

---

## Phase 9 — Monitoring & Evaluation

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

## Phase 10 — Deployment

**Goal**: the whole stack runs on Kubernetes, not just Podman locally.

- [ ] `Containerfile` per service (multi-stage builds, non-root user); build with Podman/Buildah
- [ ] `podman generate kube` from the local pods as a first-pass manifest, then harden into `infra/k8s/base/` Kustomize bases (Deployment + Service + ConfigMap/Secret) per service
- [ ] `infra/k8s/overlays/dev/` — first overlay, deploy to local `kind`/`minikube`, confirm the full graph works cluster-internally (service DNS, not `localhost`)
- [ ] Ingress (Connect-aware) as cluster entrypoint, TLS termination
- [ ] HPA on `ai-services` and `backend-services` at minimum
- [ ] `.github/workflows/ci.yml` — `buf lint`/`breaking` → `go test`/`pytest` → build/push image; `deploy.yml` — `kubectl apply -k overlays/staging` on merge to main, manual promotion gate to `overlays/prod`
- [ ] `overlays/staging` and `overlays/prod` — same manifests, different secrets/limits/replicas (MongoDB/Redis managed in prod)

**Definition of done**: a merged PR automatically builds, pushes, and rolls out to staging with zero manual steps, and you can `kubectl apply -k overlays/prod` to promote.

**Notes**:

---

## Phase 11 — Generalize into Domain Packs

**Goal**: prove the core is actually generic by extracting every IT-services-specific assumption into config. Do this only after the reference implementation works.

- [ ] Create `domain-packs/_template/` with `business.yaml`, `branding.yaml`, `system_prompt.md`, `knowledge/` per OVERVIEW.md §21
- [ ] Move all IT-services-specific content (the 14 services list, system prompt persona, seeded KB articles) out of hardcoded core files into `domain-packs/it-services/`
- [ ] `ai-services`: add a domain-pack loader — reads `tenant_id`, loads/caches the matching pack, uses `agents_enabled`/`tools_enabled` to filter what the Planner is offered
- [ ] Grep the whole `backend-services`/`ai-services`/`analysis-services` tree for remaining hardcoded IT-services strings/logic; move each into config
- [ ] Prove genericity: build a **second** domain pack for a distinct vertical (clinic / e-commerce) with its own config + a few seeded KB docs — no core code changes allowed
- [ ] `scripts/new-firm.sh` — scaffolds `_template/` into a new slug, prompts for required fields, runs KB ingestion

**Definition of done**: both domain packs run against the same core deployment, produce visibly different assistants, and the diff between them touches only `domain-packs/`.

**Notes**:

---

## Phase 12 — Multi-Tenancy

**Goal**: two firms can run on the same infrastructure without seeing each other's data.

- [ ] `tenant_id` field + index on every tenant-scoped Mongo collection; every `backend-services` RPC requires and enforces `tenant_id`
- [ ] Qdrant: one collection per tenant (start simple); ingestion and retrieval always scope by `tenant_id`
- [ ] Extend RBAC checks (`packages/shared-auth`) to be tenant-scoped: role X *within tenant Y*
- [ ] Per-tenant rate limiting and basic cost/usage tracking (token counts from Langfuse tagged by tenant) at the edge
- [ ] Isolation test: seed two tenants with overlapping data (both have a ticket "Login broken"), confirm a query scoped to tenant A never returns tenant B's document or RAG chunk

**Definition of done**: the isolation test passes, and Langfuse traces are filterable by tenant.

**Notes**:

---

## Phase 13 — Channel Adapters

**Goal**: the same assistant reachable from multiple front doors, starting with the Streamlit demo.

- [ ] **Streamlit demo** (`frontend-services/streamlit-demo/`) — `app.py` with `st.chat_message` + `st.write_stream` against `ai-services`' chat RPC (Connect/HTTP-JSON), tenant selector sidebar, `.streamlit/config.toml` dark theme + `theme.py` per `DESIGN.md` §5, `trace.py` rendering the Live Signal Path as sequential `st.columns` steps
- [ ] Containerize the Streamlit demo, add it to `podman-compose.yml`
- [ ] **Embeddable web widget** (`channels/web-widget/`) — small vanilla-JS/preact bundle, one `<script src=… data-tenant=…>` tag, opens a chat bubble, calls the chat RPC (gRPC-Web/JSON) with a scoped public API key, CORS locked to the firm's domain
- [ ] **WhatsApp adapter** (`channels/whatsapp/`) — webhook verifying Meta's challenge token, maps inbound phone-number-ID to `tenant_id`, buffers the streamed response into one outbound message
- [ ] Add a lookup collection (`channel_tenant_map`) mapping channel identifiers (WhatsApp phone number ID, widget API key) to `tenant_id`, populated by `scripts/new-firm.sh`
- [ ] End-to-end test: same question via Streamlit demo and a sandbox WhatsApp number produces consistent tool use and a correctly rendered (or plain-text-fallback) trace

**Definition of done**: a new firm can go from `./scripts/new-firm.sh acme-clinic` to a working Streamlit demo *and* a working WhatsApp number in one sitting, zero core code changes.

**Notes**:

---

## Phase 14 — Open-Source Release Readiness

**Goal**: a stranger can find this repo, understand it, run it, and trust it enough to point it at their business.

- [ ] `LICENSE` (MIT)
- [ ] Public root `README.md` rewritten for the OSS audience: elevator pitch, quickstart (`podman-compose up` → open Streamlit demo in under 10 minutes), supported channels, screenshot/GIF of the trace UI, links to `docs/architecture/OVERVIEW.md`, `PLAN.md`, `DESIGN.md`, license badge
- [ ] `CONTRIBUTING.md` — how to run tests, and the extension points (domain pack / channel / MCP server / data-access domain / analysis module), linking back to `CLAUDE.md`
- [ ] `CODE_OF_CONDUCT.md` (Contributor Covenant), `SECURITY.md` (private vulnerability reporting)
- [ ] `CHANGELOG.md` (Keep a Changelog format), tag `v0.1.0`; `CHECKLIST.md` version log kept current
- [ ] `.github/ISSUE_TEMPLATE/` — bug report, new-domain-pack request, new-channel request; `.github/PULL_REQUEST_TEMPLATE.md`
- [ ] Add `core_version` to `business.yaml` schema; `buf breaking` guarding contract compatibility in CI
- [ ] Full design-system pass: confirm Next.js app, Streamlit demo, and web widget all read as one visual product per `DESIGN.md`

**Definition of done**: a first-time visitor can go from README to a running Streamlit demo without reading any other doc, and knows exactly where to look (`CONTRIBUTING.md`) to add their own domain pack or channel.

**Notes**:

---

## Parking lot (explicitly out of scope until the phases above are done)
- Multi-tenant billing / metered usage
- SSO / SAML for enterprise admin accounts
- Fine-tuning or distillation of a smaller in-house model
- Non-English language support
- Promoting an `analysis-services` module to its own scalable service (only if profiling demands it)

Add things here instead of scope-creeping into an active phase.
