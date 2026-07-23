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
- [~] `buf.yaml` + `buf.gen.yaml` generating stubs — Go done; Python targets land in Phase 2 with `ai-services`
- [~] `buf lint` clean and passing; `buf breaking` + CI wiring comes with Phase 13 CI/CD
- [x] `protos/database/v1/` shared value types (`PageRequest`/`PageResponse`, `ErrorDetail`, `TenantScope`, `Money`, `Health`)
- [x] `is_collection: true` convention adopted — every collection-backed entity lives in `protos/database/v1/<entity>.proto`, `data_access` protos import it and hold RPC-only messages, and the entity's first field is `_id` (Mongo's real primary key, Go field `XId`) (reference: `protos/database/v1/ticket.proto`)
- [~] `protos/database/v1/` entity schemas + `protos/backend_services/data_access/v1/` RPC contracts — `ticket` done, core domains (`chat`/`knowledge-base`/`calendar`/`auth`) in Phase 1, rest in Phase 10 (see §2.1)
- [ ] `protos/ai_services/v1/chat.proto` (server-streaming)
- [ ] `protos/analysis_services/v1/` capabilities

### Infra & tooling
- [x] `infra/podman-compose.yml` — mongo, redis, qdrant, minio, backend-services; **runtime-verified 2026-07-23**, all healthy via `podman-compose up -d` (see PLAN.md Phase 0 notes for the WSL2 cgroup v2 fix + the podman-compose build-path workaround: backend-services is a pre-built `image:`, not an inline `build:`)
- [x] `infra/containerfiles/` per service — `backend-services.Containerfile` done, builds clean (`make build-backend-image`)
- [x] `.env.example` complete
- [x] Root `Makefile` (`make up/gen/down/logs/lint/build-backend/test-backend`)
- [ ] `packages/shared-auth` — JWT-verify Connect interceptor + RBAC helper (Go + Python)
- [ ] `packages/shared-clients` — pre-wired Connect clients to backend/analysis
- [ ] `packages/proto-stubs` — re-exports of generated stubs

---

## 2. Service groups

### 2.1 `backend-services` (Go) — the only DB tier
RPC-per-collection layout (proto entity + mongodb/<collection>.go + rpc_services/<collection>/ + index bootstrap) — see `backend-services/CLAUDE.md`:
- [x] `mongodb/initialize.go` (single Mongo client + `DbType`/`Queries`/`Db` — the only DB creds in the repo)
- [x] `main.go` at module root (Connect server, `Health` RPC via `health/`) — builds, vets, and runs clean against a live Mongo instance
- [x] **ticket** — `mongodb/ticket.go` + `rpc_services/ticket/`: `CreateTicket`, `GetTicket`, `ListTickets` — **runtime-verified 2026-07-23** via `buf curl` against the compose-launched instance, confirmed persisted with `mongosh`. Reference example for the `_id`-as-primary-key convention.
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
- Stay on `0.x` until Phase 13 (deployment + open-source release readiness); tag **`v1.0.0`** when the full reference stack (including the frontend) + both domain packs run end-to-end and the OSS surface is complete.

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
| 1 | Backend core domains (`chat`, `knowledge-base`, `calendar`, `auth`) | `v0.2.0` |
| 2 | AI Services core: streaming chat, tool calling, Streamlit dev tool | `v0.3.0` |
| 3 | Channels (web widget, WhatsApp, Slack) | `v0.4.0` |
| 4 | RAG with citations | `v0.5.0` |
| 5 | Memory across sessions | `v0.6.0` |
| 6 | Multi-agent Planner routing | `v0.7.0` |
| 7 | Grouped compute (`analysis-services`) | `v0.8.0` |
| 8 | First MCP server + client (Calendar) | `v0.9.0` |
| 9 | Observability & evaluation | `v0.10.0` |
| 10 | Backend completion (remaining 8 domains) + RBAC everywhere | `v0.11.0` |
| 11 | Domain packs + multi-tenancy (isolation test passing) | `v0.12.0` |
| 12 | Frontend — Next.js web app | `v0.13.0` |
| 13 | Deployment (Kubernetes) + OSS release readiness | **`v1.0.0`** |

---

## 4. Version log

Newest first. One row per released version.

| Version | Date | Phase | Highlights |
|---|---|---|---|
| **`v0.1.0`** | 2026-07-23 | 0 | Phase 0 complete: `podman-compose up -d` brings up healthy Mongo/Redis/Qdrant/MinIO/backend-services, `CreateTicket`/`GetTicket`/`ListTickets` verified end-to-end against real MongoDB via `buf curl` + `mongosh`. Module 0's 4 `LEARNING.md` topic docs written (`docs/roadmap/00-contracts-data/`). |

> When you cut a release: add a row here, add a matching `CHANGELOG.md` entry, tag the commit (`git tag v0.x.0`), and make sure the phase's boxes above are all `[x]`.
