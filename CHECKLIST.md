# Implementation Checklist & Version Log — ServiceSphere AI

The **cumulative state of what is actually built**, and the **release history**. `PLAN.md` is the ordered build plan (what to do next); this file is the running scoreboard (what exists now) plus versioning.

How to use it:
- When you finish something real (a service, an RPC, a tool, a doc), check its box here **and** in `PLAN.md`/`LEARNING.md`.
- A box is `[x]` only when the thing runs and is wired end-to-end — not when a stub exists. Use `[~]` for in-progress/partial.
- Cut a version entry in §4 when a phase's "Definition of done" is met (see the versioning scheme in §3).
- Keep this honest. An unchecked box is information; a falsely-checked one is a bug waiting for the next session.

Legend: `[ ]` not started · `[~]` in progress / partial · `[x]` done and wired end-to-end.

---

## 1. Foundations & cross-cutting

### Contracts (`protos/` + buf)
- [ ] `buf.yaml` + `buf.gen.yaml` generating Go + Python stubs
- [ ] `buf lint` + `buf breaking` wired into CI
- [ ] `protos/database/v1/` shared types (`Page`, `ErrorDetail`, `TenantScope`, `Money`)
- [ ] `protos/backend_services/data_access/v1/` domains defined (see §2.1)
- [ ] `protos/ai_services/v1/chat.proto` (server-streaming)
- [ ] `protos/analysis_services/v1/` capabilities

### Infra & tooling
- [ ] `infra/podman-compose.yml` — mongo, redis, qdrant, minio, langfuse healthy
- [ ] `infra/containerfiles/` per service
- [ ] `.env.example` complete
- [ ] Root `Makefile`/`justfile` (`make up/gen/down/logs`)
- [ ] `packages/shared-auth` — JWT-verify Connect interceptor + RBAC helper (Go + Python)
- [ ] `packages/shared-clients` — pre-wired Connect clients to backend/analysis
- [ ] `packages/proto-stubs` — re-exports of generated stubs

---

## 2. Service groups

### 2.1 `backend-services` (Go) — the only DB tier
Data-access domains (proto + repository + index bootstrap + Connect handler + `Health`):
- [ ] `database/mongo.go` (single Mongo client — the only DB creds in the repo)
- [ ] `data-access/cmd/server` (Connect server, `Health` RPC)
- [ ] **ticket** — `CreateTicket`, `GetTicket`, `ListTickets`
- [ ] **chat** — `AppendMessage`, `GetSession`, `WriteMemory`, `GetPreferences`
- [ ] **knowledge-base** — `ListArticles`, `UpsertArticle`
- [ ] **calendar** — `GetAvailability`, `BookMeeting`
- [ ] **auth** — `Register`, `Login`, `Refresh`, `AssignRole` + JWT issuing
- [ ] **customer** — `GetCustomer`, `UpsertCustomer`
- [ ] **crm** — `CreateLead`, `UpdateDealStage`
- [ ] **project** — `GetProjectStatus`, `AddMilestone`
- [ ] **invoice** — `CreateInvoice`, `ListInvoices` (+ `idempotency_key`)
- [ ] **proposal** — `CreateProposal`, `AddProposalVersion`
- [ ] **notification** — `EnqueueNotification`
- [ ] **analytics** — `GetOverview`, `PutSnapshot`
- [ ] **document** — `CreateDocument`, `GetDocument` (MinIO keys)
- [ ] Tenant scoping + RBAC enforced on every RPC

### 2.2 `ai-services` (Python) — orchestration
- [ ] `server/chat_service.py` — Connect streaming chat RPC + `Health`
- [ ] `llm/` provider abstraction (Ollama + OpenAI-compatible)
- [ ] `graph/` — `state.py`, `planner.py`, `router.py`, `build_graph.py`
- [ ] `agents/` — planner, sales, support, finance, project, knowledge, rag, calendar, email, document, memory, evaluation
- [ ] `tools/` — `createTicket`, `createLead`, `getInvoices`, `estimateProject`, `generateProposal`, `bookMeeting`, `uploadDocument`, `summarizeDocument`, `searchKnowledge`, `sendEmail`, `trackProject`, `generateInvoice`, `getAnalytics`
- [ ] `rag/` — chunking, embed, retrieve, rerank
- [ ] `memory/` — short-term (Redis) + long-term (via backend RPC + Qdrant)
- [ ] `mcp_clients/` — per-server wrappers
- [ ] `evals/` — golden dataset + harness
- [ ] Holds **zero** DB credentials (all data via `backend-services` RPC)

### 2.3 `analysis-services` (Python) — grouped compute
- [ ] `server/app.py` — the ONE common Connect server + `Health`
- [ ] `estimation/` module (`estimateProject`)
- [ ] `analytics/` module (`getAnalytics` aggregations)
- [ ] `scoring/` module (rerank / faithfulness)
- [ ] Modules call `backend-services` for any data (no DB access)

### 2.4 `frontend-services`
- [ ] `web/` Next.js skeleton (TS + Tailwind + shadcn) + Connect client
- [ ] `web/` chat UI (streaming, tool-call cards, Live Signal Path)
- [ ] `web/` dashboards (customer + admin)
- [ ] `streamlit-demo/` — `app.py`, `theme.py`, `trace.py`, dark theme, tenant selector

### 2.5 `channels`
- [ ] `web-widget/` — embeddable `<script>` bubble, scoped API key, CORS
- [ ] `whatsapp/` — webhook, phone-number-ID → tenant, buffered response
- [ ] `slack/` (optional)
- [ ] `channel_tenant_map` lookup collection

### 2.6 `mcp-servers`
- [ ] `calendar-mcp/`
- [ ] `email-mcp/`
- [ ] `filesystem-mcp/`
- [ ] `knowledge-mcp/`
- [ ] `document-mcp/`
- [ ] `analytics-mcp/`
- [ ] `github-mcp/`
- [ ] `browser-mcp/`

### 2.7 Generalization
- [ ] `domain-packs/_template/`
- [ ] `domain-packs/it-services/` (reference pack extracted from core)
- [ ] Second vertical domain pack (proves genericity)
- [ ] Domain-pack loader in `ai-services`
- [ ] `scripts/new-firm.sh`
- [ ] Multi-tenancy: `tenant_id` on every collection + isolation test passing

### 2.8 Observability & deployment
- [ ] Langfuse tracing across `ai-services` (LLM/tool/RPC spans)
- [ ] OpenTelemetry across Go + Python (gRPC/Connect interceptors) → Grafana
- [ ] Eval harness in CI
- [ ] `Containerfile` per service (multi-stage, non-root)
- [ ] `infra/k8s/base/` Kustomize bases + `overlays/{dev,staging,prod}`
- [ ] CI/CD workflows (buf → test → build → deploy)

---

## 3. Phase completion & versioning scheme

**Versioning** follows [Semantic Versioning](https://semver.org): `MAJOR.MINOR.PATCH`.

Pre-1.0 (where this project lives while the reference implementation is being built):
- **MINOR** (`0.x.0`) — bump when a `PLAN.md` phase reaches its "Definition of done". One phase ≈ one minor release.
- **PATCH** (`0.x.y`) — fixes, doc updates, and additive work within a phase that doesn't complete a new phase.
- Stay on `0.x` until Phase 14 (open-source release readiness); tag **`v1.0.0`** when the full reference stack + both domain packs run end-to-end and the OSS surface is complete.

Post-1.0:
- **MAJOR** — a breaking change to a published contract in `protos/` (guarded by `buf breaking`), or a break in the domain-pack (`business.yaml`) schema.
- **MINOR** — new backward-compatible capability (new RPC, new tool, new channel, new domain pack).
- **PATCH** — backward-compatible fixes.

Rules:
- A phase's version is cut only after its `LEARNING.md` docs are written and its `PLAN.md` "Definition of done" is checked.
- Contract compatibility is enforced in CI by `buf breaking`; a MAJOR bump is the only way to ship an intentional break.
- Every release gets a row in §4 and an entry in `CHANGELOG.md` (Keep a Changelog format).
- A domain pack declares the core version it targets via `business.yaml: core_version`.

Phase → planned version:

| Phase | Milestone | Planned version |
|---|---|---|
| 0 | Contracts + Go/Mongo data tier, `CreateTicket` writes to Mongo | `v0.1.0` |
| 1 | Streaming chat RPC end to end (Ollama) | `v0.2.0` |
| 2 | Tool calling → `backend-services` (real ticket via tool) | `v0.3.0` |
| 3 | RAG with citations | `v0.4.0` |
| 4 | Memory across sessions | `v0.5.0` |
| 5 | First MCP server + client (Calendar) | `v0.6.0` |
| 6 | Multi-agent Planner routing | `v0.7.0` |
| 7 | Grouped compute (`analysis-services`) | `v0.8.0` |
| 8 | Production contracts + auth (all domains, RBAC) | `v0.9.0` |
| 9 | Monitoring & evaluation | `v0.10.0` |
| 10 | Podman → Kubernetes deployment | `v0.11.0` |
| 11 | Domain packs (generalized core) | `v0.12.0` |
| 12 | Multi-tenancy (isolation test passing) | `v0.13.0` |
| 13 | Channel adapters (Streamlit, widget, WhatsApp) | `v0.14.0` |
| 14 | OSS release readiness | **`v1.0.0`** |

---

## 4. Version log

Newest first. One row per released version.

| Version | Date | Phase | Highlights |
|---|---|---|---|
| _(unreleased)_ | — | 0 | Pre-implementation: docs and architecture defined; no code yet |

> When you cut a release: add a row here, add a matching `CHANGELOG.md` entry, tag the commit (`git tag v0.x.0`), and make sure the phase's boxes above are all `[x]`.
