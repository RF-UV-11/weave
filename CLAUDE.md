# ServiceSphere AI — Project Instructions

Open-source, multi-tenant AI assistant backbone. Multi-agent (LangGraph), RAG-backed, MCP-integrated. The core is domain-agnostic — a **domain pack** (`domain-packs/<firm>/`) is what turns it into a specific firm's assistant, delivered over whichever **channels** (`channels/`) that firm needs: their own web app, an embeddable widget, WhatsApp, or the Streamlit demo. The IT-services domain pack built in the early phases is the reference example, not a hardcoded assumption — don't bake IT-services-specific logic into `backend-services/`, `ai-services/`, or `analysis-services/` core code; it belongs in the domain pack config.

Full architecture: `docs/architecture/OVERVIEW.md`. Design system (applies to every UI surface, every channel): `DESIGN.md`. Build order: `PLAN.md` — **always check PLAN.md for the current phase and next unchecked task before starting work.** Learning curriculum (module/topic map + where each concept's doc belongs): `LEARNING.md`.

## Architecture in one paragraph
Services are grouped into **repos by concern** and communicate over **gRPC** (Connect protocol). **`protos/`** is the single source of truth for every service contract. **`backend-services/` (Go) is the *only* layer allowed to touch MongoDB** — every other service that needs data calls it over gRPC; nothing else opens a Mongo connection. **`ai-services/` (Python)** runs the LangGraph orchestration (agents, tools, RAG, memory). **`analysis-services/` (Python)** is a *grouped* compute service: one common server exposes endpoints, and each calculation/analysis capability is an internal module reached via a route — not a separately deployed service. Browsers and channels reach the system through **Connect** (HTTP/JSON + gRPC-Web from the same handlers, no proxy). Deployment is local-first via **Podman**.

## Tech stack (don't substitute without asking)
- **Contracts**: Protocol Buffers in `protos/`, codegen via `buf` → Go structs + Python classes. gRPC / Connect (connectrpc) is the transport for *all* inter-service calls.
- **Data layer**: Go 1.23 + `connect-go`, official MongoDB Go driver. This is the only tier with DB credentials.
- **AI orchestration**: Python 3.12, LangGraph for orchestration, LangChain only for loaders/splitters, Pydantic v2 for all tool I/O schemas + structured LLM output + config. Python services expose Connect endpoints via `connecpy`; internal calls use gRPC. LLMs via Ollama (local) or any OpenAI-compatible API (cloud).
- **Compute/analysis**: Python 3.12, same Connect server pattern — one server per group, capabilities as modules.
- **Frontend**: Next.js 14 (App Router) + TypeScript + Tailwind + shadcn/ui (`frontend-services/web`); Streamlit reference demo (`frontend-services/streamlit-demo`). Browser talks Connect (HTTP/JSON or gRPC-Web) directly.
- **Database**: MongoDB (via `backend-services` only) · Cache/queue: Redis 7 · Vector: Qdrant (Chroma for local dev only) · Object storage: MinIO
- **Observability**: Langfuse (LLM/agent traces) + OpenTelemetry (infra metrics)
- **Auth**: JWT (access+refresh) in Connect request metadata, verified by a shared interceptor + RBAC
- **Infra**: Podman + `podman-compose` locally, Kubernetes (Kustomize overlays; `podman generate kube` as a starting point) in prod

## Repo layout
```
protos/                   Single source of truth for ALL gRPC/Connect contracts (codegen with buf)
  database/                       EVERY schema that is stored, one .proto per entity — collection-backed
                                   messages carry an `is_collection: true` comment (see convention below)
  backend_services/data_access/   RPC contracts ONLY (Request/Response + service); no entity fields live
                                   here — they import and reference the matching message in database/v1
  ai_services/                    chat/orchestration RPCs (streaming)
  analysis_services/              analysis/compute RPCs
backend-services/         GO — the ONLY layer that touches MongoDB
  data-access/            Connect/gRPC servers, one package per domain (tickets, invoices, ...)
  database/               Mongo client, collections, repositories, index bootstrap
  internal/               shared Go: auth interceptor, tracing, middleware
  gen/                    generated Go stubs from protos/
ai-services/              PY — LangGraph orchestration; calls backend-services over gRPC for all data
  server/                 common Connect server = the chat entrypoint (streaming)
  graph/ agents/ tools/ rag/ memory/ mcp_clients/ llm/ evals/
  gen/                    generated Python stubs from protos/
analysis-services/        PY — grouped compute: ONE common server + capability modules via routes
  server/                 the only entrypoint; endpoints are the shared surface
  <capability>/           each calc/analysis capability as a module (NOT a separate deploy)
frontend-services/        All UI surfaces
  web/                    Next.js customer + admin portal
  streamlit-demo/         Reference "try it live" channel
channels/*/               Thin adapters: web-widget, whatsapp, slack — channel-native <-> core chat RPC only
domain-packs/*/           Per-firm config: business.yaml, branding.yaml, system_prompt.md, knowledge/
mcp-servers/*/            Standalone MCP servers (own process, own README per server)
packages/                 Shared: re-exported stubs, auth interceptor, Connect clients, shared schemas
infra/                    Podman (podman-compose), k8s manifests, observability config
docs/architecture/        Full design doc + diagrams (source of truth for "why")
docs/roadmap/             Module/topic learning docs, theory + examples — structure defined in LEARNING.md
scripts/new-firm.sh       Scaffolds a new domain-packs/<firm>/ from _template/
```

## Non-negotiable conventions
- **`backend-services` is the data trust boundary. Nothing else touches MongoDB.** No Mongo connection string, driver, or query lives anywhere except `backend-services/`. `ai-services`, `analysis-services`, channels, and MCP servers all read/write data by calling a `backend-services` RPC. If you catch yourself importing a Mongo driver outside `backend-services/`, stop — add/extend a data-access RPC instead.
- **Tools are the trust boundary for the LLM.** The LLM never generates queries, shell commands, or raw RPC. It only calls a typed Python tool function with a Pydantic input/output schema. Tools call `backend-services` (data) or `analysis-services` (compute) or MCP servers over gRPC — never a DB.
- **`protos/` is the contract source of truth.** Every RPC starts as a `.proto` in `protos/`. Regenerate stubs with `buf generate`; never hand-edit generated code in `gen/`. Wire types come from protobuf; Pydantic mirrors are only for LLM-facing tool schemas and config, not the wire.
- **Every stored schema lives in `protos/database/v1/`, one entity per file, never inline in a `data_access` proto.** A message that backs a MongoDB collection carries this exact comment immediately above the `message` keyword:
  ```protobuf
  // is_collection: true
  // Mongo collection "<name>", owned by backend-services/database/repositories/<name>_repo.go.
  message Ticket { ... }
  ```
  `data_access` protos import the entity from `database/v1` and define RPC-only messages (`CreateTicketRequest`, etc.) — they never redeclare entity fields. A message in `database/v1` that is *not* collection-backed (e.g. a value type embedded in one) simply omits the flag — don't write `is_collection: false`. See `protos/database/v1/ticket.proto` for the reference example.
- **Grouped compute stays grouped.** A new calculation/analysis capability is a *module + route* inside `analysis-services`, not a new top-level service. Only promote it to its own service if it needs independent scaling/deploy — and ask first.
- **Every service** exposes a Connect server with a `Health` RPC (or `grpc.health.v1`), owns its slice of `protos/`, and — for data — a repository + index bootstrap in `backend-services/database/`.
- **Every agent/tool/RPC hop is traced in Langfuse / OpenTelemetry.** Don't add a new agent node or cross-service RPC without a span.
- **API/contract conventions**: proto packages are versioned (`...v1`). Errors use Connect/gRPC status codes carrying a structured `ErrorDetail{code, message, details}`. List RPCs are cursor-paginated (`page_token`/`page_size`). RPCs that create billable resources (invoices, payments) take an `idempotency_key`.
- **RBAC is re-checked at the tool layer**, not just at the edge — a tool like `generateInvoice` must independently verify the caller's role before executing, since the LLM's intent to call it isn't authorization.
- Prefer editing/extending existing files in a phase over creating parallel "v2" versions.
- **Core stays generic.** Business-specific logic (which services a firm offers, tone, enabled agents/tools) belongs in `domain-packs/<firm>/`, never hardcoded in `backend-services/`, `ai-services/`, or `analysis-services/`. If you catch yourself writing `if tenant == "it-services"` in core code, extend the config schema in `domain-packs/_template/` instead.
- **Channels stay thin.** `channels/*` and `frontend-services/streamlit-demo` only translate to/from the core chat RPC on `ai-services`. If a channel adapter needs to know about tools, agents, or the DB, that logic is misplaced — it belongs in `ai-services`.
- **Every UI surface is dark-mode-first per `DESIGN.md`.** No light-only screens, no ad hoc color choices outside the token table — including the Streamlit demo and any embeddable widget CSS.
- **Multi-tenant by default.** Any Mongo collection storing firm-specific data gets a `tenant_id` field + index; any Qdrant collection or memory write is scoped to a tenant. No "we'll make it multi-tenant later" shortcut.

## Commands
```bash
podman-compose -f infra/podman-compose.yml up -d        # full local stack: mongo, redis, qdrant, minio, langfuse, services
podman-compose -f infra/podman-compose.yml up -d <svc>  # bring up one service + its deps
buf lint && buf generate                                # validate protos + regenerate Go/Python stubs
go run ./backend-services/data-access/cmd/server        # run the Go data-access server locally
python -m ai_services.server                            # run the AI orchestration server locally
python -m analysis_services.server                      # run the grouped compute server locally
pnpm dev                                                # frontend, from frontend-services/web
streamlit run app.py                                    # Streamlit demo, from frontend-services/streamlit-demo
./scripts/new-firm.sh <slug>                            # scaffold a new domain-packs/<slug>/ from _template/
go test ./...                                           # Go tests, from backend-services/
pytest                                                  # Python tests, from a Python service dir
golangci-lint run                                       # lint Go
ruff check . && ruff format .                           # lint/format Python
pnpm lint && pnpm typecheck                             # frontend
```
(These commands are targets, not yet all wired — Phase 1 sets up the actual scripts/Makefile. Update this section once real commands exist.)

## Working style for this repo
- Work phase by phase per `PLAN.md`. Don't jump ahead to MCP tooling while basic tool-calling tasks are still unchecked.
- After finishing a task, update the checkbox in `PLAN.md` and note anything the next session needs (real command, gotcha, decision made) — in `PLAN.md`'s phase notes or `docs/roadmap/phase-N.md` if it's substantial.
- **Before marking a phase's "Definition of done" complete in `PLAN.md`, generate/update every topic doc `LEARNING.md` lists for that phase's module**, following the template in `LEARNING.md` §1 — full theory + a worked example grounded in the code you just wrote, not a stub. Flip its status to `[x]` in `LEARNING.md` §2 once written.
- When a design decision in `docs/architecture/OVERVIEW.md` turns out to be wrong once you build it, update that doc in the same commit — don't let the doc drift from reality.
- Ask before adding a new top-level service group, a new standalone service (vs. a module inside an existing group), a new top-level dependency, or deviating from the tech stack above.
