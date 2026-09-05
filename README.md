# Weave

**Weave your systems into one AI assistant.**

Weave is an open-core platform that turns whatever a business or individual already runs — a CRM, a helpdesk, a calendar, an inventory system, a personal inbox, a notes app — into a conversational AI assistant that can actually *act* on it. No data migration, no forking a codebase, no rebuilding your workflows inside someone else's system.

You connect your systems as **MCP servers** (self-hosted, or scaffolded in minutes from a template), define one or more **bot profiles** (a customer-facing assistant, a staff-facing one, or just "you"), and deploy it wherever people should reach it — a website widget, WhatsApp, Slack, or a raw API. Weave's orchestration brain — a planner, specialist agents, retrieval, and memory — handles the reasoning. Your data never leaves your systems.

## Why

Most AI-assistant products want your data inside their database. Weave inverts that: it's the *connective layer*, not the system of record. A tenant in Weave is just an identity plus a set of registered connectors plus a bot profile — which means the same core serves a company **and** an individual with a personal assistant wired to their own accounts, without special-casing either.

## How it works

1. **Connect** — register one or more MCP servers exposing your tools/data (write your own, or scaffold one from a connector template).
2. **Configure** — define a bot profile: persona, which connectors/tools it can use, which channels it's reachable on, which roles can reach it.
3. **Deploy** — drop it behind your web app, an embeddable widget, WhatsApp, Slack, or call the chat API directly.

Weave resolves the tenant and active bot profile per request, assembles the available tools dynamically from that tenant's registered connectors, and routes through a planner → specialist agents → tools loop — every hop traced end-to-end.

## Status

**Pre-alpha, but no longer "nothing is built."** The core mechanic — dynamic tool assembly, per-bot-profile persona/guardrails/LLM-provider choice, per-tool visibility and per-end-user auth, session + cross-session memory, a real web chat/admin UI — is built and live-verified end-to-end against a real running stack (see `PLAN.md` for the phase-by-phase record, including what's honestly *not* yet verified). What "pre-alpha" still means: no published package (`weave-sdk` is a path/git dependency, not yet on PyPI), no license chosen yet, and several `docs/architecture/SECURITY.md`-tracked hardening gaps (credential rotation, service-to-service auth) before any real non-dev tenant should be onboarded. Two fully worked reference integrations exist as independent sibling projects — see [`OVERVIEW.md`](OVERVIEW.md) §6 and [`docs/guides/DEMO_TENANTS.md`](docs/guides/DEMO_TENANTS.md).

## Repository layout

This is a monorepo. See `OVERVIEW.md` for the full picture; current structure:

```
core/           Go — the only tier holding platform data (tenants, connector registry,
                credential vault, chat/session/memory, auth, billing/usage)
orchestrator/   Python — chat gRPC server, LangGraph planner/agents, MCP client,
                dynamic tool assembly, per-bot-profile LLM provider routing, RAG/memory
mcp-gateway/    Python — turns a tenant's registered HTTP tools (core.HttpTool) into
                a real MCP server, per tenant, dynamically — no MCP server for a
                tenant to run themselves
connectors/     Weave's own reference/scaffolding MCP servers (reference-mcp,
                dev-stub-mcp) — never a tenant's actual business code, which lives
                in the tenant's own repo (see "Demo tenants" below)
web/            Next.js — chat UI, admin console (bot profiles/tools/connectors),
                grpc-web to orchestrator/core through an Envoy proxy
packages/       weave-sdk (the tenant-facing SDK — self-contained, installable in
                any external project on its own) + shared-auth/shared-clients/
                shared-ratelimit (internal-platform-only, never a tenant dependency)
protos/         Every service contract (Protocol Buffers)
infra/          Local (Podman Compose) and cluster (Kubernetes) deployment
docs/           Architecture and security documentation
```

`channels/`, `compute/`, and `packs/` (thin per-channel adapters, platform-generic compute modules, vertical starter templates) are designed but not yet built — see `PLAN.md`'s "Phase 4+" for what's actually planned there, rather than treating an empty directory as documentation of intent.

**Demo tenants live outside this repo entirely** — a tenant's integration is their own code in their own codebase, same as any real customer's would be. Two independent reference projects, each its own git repo, install `weave-sdk` and walk the same onboarding flow: `tarang-electronics` (Indian consumer electronics, B2C) and `suvidha-finserve` (Indian accounting/bookkeeping, B2B) — see `OVERVIEW.md` §6 and [`docs/guides/DEMO_TENANTS.md`](docs/guides/DEMO_TENANTS.md) for what each demonstrates and how to run them. New tenants configuring their own integration should start with [`docs/guides/ONBOARDING.md`](docs/guides/ONBOARDING.md).

## Documentation

- [`docs/WEAVE_FROM_SCRATCH.md`](docs/WEAVE_FROM_SCRATCH.md) — **the full-depth read**: the idea and the need, why not the alternatives, every architectural decision and what was rejected, the security model, and a topic-by-topic curriculum for building the whole system from scratch
- [`OVERVIEW.md`](OVERVIEW.md) — what Weave is, the core mechanic, tech stack, repo structure
- [`docs/architecture/ARCHITECTURE.md`](docs/architecture/ARCHITECTURE.md) — system architecture, request lifecycle, data model, dynamic tool assembly (bulk/spec-driven registration, per-profile persona/LLM-provider, per-tool end-user auth)
- [`docs/architecture/SECURITY.md`](docs/architecture/SECURITY.md) — trust boundaries, tenant isolation, credential handling, connector security
- [`docs/guides/ONBOARDING.md`](docs/guides/ONBOARDING.md) — step-by-step configuration guide for onboarding a new tenant (sign up → connect a channel), with flow diagrams
- [`docs/guides/DEMO_TENANTS.md`](docs/guides/DEMO_TENANTS.md) — the two reference tenant integrations (`tarang-electronics`, `suvidha-finserve`): what each demonstrates and how to run them end-to-end
- [`PLAN.md`](PLAN.md) — the phase-by-phase build record: what's done, how each phase was live-verified, and honest gaps
- [`DESIGN.md`](DESIGN.md) — visual identity and UI design system

## License

TBD — this repo is pre-alpha; a license will be added before any public release.
