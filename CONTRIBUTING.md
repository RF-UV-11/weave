# Contributing to ServiceSphere AI

Thanks for considering a contribution. This project has a few well-defined extension points — most contributions fall into one of them. Whatever you touch, the four spine rules hold: services are grouped repos talking over **gRPC/Connect**, `protos/` is the only contract source, **`backend-services` (Go) is the only tier that touches MongoDB**, and `analysis-services` is one grouped compute server (capabilities are modules-via-routes, not new deployments).

## The extension points

### 1. Add a domain pack
The best way to prove (and improve) the core's genericity. Copy `domain-packs/_template/` to `domain-packs/<your-example>/`, fill in `business.yaml`, `branding.yaml`, `system_prompt.md`, and a small `knowledge/` set for a vertical not yet represented (retail, legal, real estate, education...). Domain packs are config only — if you find yourself needing to touch service code to make your pack work, that's a bug in the core's genericity, not a reason to fork it. Open an issue describing the gap instead.

### 2. Add a channel adapter
Channels live in `channels/<name>/` (and full UI surfaces in `frontend-services/`) and do exactly one job: translate a channel-native message into a call to `ai-services`' streaming chat RPC, and translate the response back. See `docs/architecture/OVERVIEW.md` §22 for the existing adapters. A new channel PR should not add business logic — if it needs to know about tools, agents, or the database, that belongs in `ai-services`.

### 3. Add an MCP server
New MCP servers live in `mcp-servers/<name>/` with `server.py`, `resources.py`, `tools.py`, `prompts.py`, and their own `README.md` documenting the contract. See `docs/architecture/OVERVIEW.md` §8. When your MCP server needs firm data, it calls a `backend-services` RPC — it must not open a database connection.

### 4. Add a data-access domain (`backend-services`, Go)
New persisted data means a new (or extended) domain in `backend-services/`: a `.proto` in `protos/backend_services/data_access/v1/`, a repository + index bootstrap in `backend-services/database/`, and a Connect handler. This is the *only* place a MongoDB driver is allowed. Regenerate stubs with `buf generate`.

### 5. Add an analysis/compute capability (`analysis-services`, Python)
A calculation or analysis capability is a **module + route inside the one `analysis-services` server** — not a new top-level service. Add a `.proto` in `protos/analysis_services/v1/`, a module folder, and register its route on the common server. If your capability needs firm data, it calls `backend-services`. Only propose promoting a module to its own service if profiling shows it needs independent scaling.

## Conventions

Development conventions (tech stack, the data trust boundary, the tool-calling trust boundary, contract/API standards, RBAC rules) live in [`CLAUDE.md`](CLAUDE.md) — that file is the single source of truth whether you're a human or a coding agent. Read it before your first PR.

Design conventions (color tokens, typography, the Live Signal Path trace element) live in [`DESIGN.md`](DESIGN.md) — any PR touching UI in any channel should follow it, including the Streamlit demo.

## Before opening a PR

- [ ] Contracts are clean: `buf lint` and `buf breaking` pass; generated stubs in `gen/` are regenerated, not hand-edited
- [ ] Tests pass: `go test ./...` in `backend-services/`, `pytest` in the relevant Python service dir, `pnpm test` for frontend
- [ ] Lint passes: `golangci-lint run` for Go, `ruff check .` for Python, `pnpm lint && pnpm typecheck` for frontend
- [ ] No new MongoDB access outside `backend-services/` — data reaches other tiers only via RPC
- [ ] If you added a collection, there's a repository + index bootstrap in `backend-services/database/migrate/`, and (if tenant-scoped) a `tenant_id` field + index
- [ ] If you touched a design surface, it matches `DESIGN.md` — no ad hoc colors or fonts outside the token table
- [ ] If your change affects the architecture in `docs/architecture/OVERVIEW.md`, update that doc in the same PR
- [ ] Update `CHECKLIST.md` (and the relevant `PLAN.md` checkbox / `LEARNING.md` status) for anything you completed

## Reporting bugs / requesting features

Use the issue templates under `.github/ISSUE_TEMPLATE/` — separate templates for bug reports, new domain-pack requests, and new channel requests, since they need different information.

## Security issues

Do not open a public issue for a security vulnerability. See [`SECURITY.md`](SECURITY.md).

## Code of conduct

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md). Be kind.
