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

**Pre-alpha — fresh start.** This repo was re-baselined from an earlier services-specific prototype into a general, MCP-first platform. Nothing is built yet; see `docs/architecture/` for the current design.

## Repository layout

This is a monorepo. See `OVERVIEW.md` for the full picture; short version:

```
core/           Go — the only tier holding platform data (tenants, connector registry,
                credential vault, chat/session/memory, auth, billing)
orchestrator/   Python — LangGraph planner/agents, MCP client, dynamic tool assembly, RAG, memory
compute/        Python — optional, platform-generic compute modules (not tenant business logic)
connectors/     Reference MCP server(s) + connector templates/SDK
channels/       Thin adapters: web widget, WhatsApp, Slack, raw API
web/            Next.js — onboarding dashboard, admin, chat UI, embeddable widget
packs/          Tenant config + vertical starter templates
packages/       Shared libraries (auth, clients, connector SDK)
protos/         Every service contract (Protocol Buffers)
infra/          Local (Podman) and cluster (Kubernetes) deployment
docs/           Architecture, security, and design documentation
```

## Documentation

- [`OVERVIEW.md`](OVERVIEW.md) — what Weave is, the core mechanic, tech stack, repo structure
- [`docs/architecture/ARCHITECTURE.md`](docs/architecture/ARCHITECTURE.md) — system architecture, request lifecycle, data model
- [`docs/architecture/SECURITY.md`](docs/architecture/SECURITY.md) — trust boundaries, tenant isolation, credential handling, connector security
- [`DESIGN.md`](DESIGN.md) — visual identity and UI design system

## License

TBD — this repo is pre-alpha; a license will be added before any public release.
