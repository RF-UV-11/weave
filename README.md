<div align="center">

# ServiceSphere AI

**One AI assistant for every customer interaction — open source, multi-tenant, channel-agnostic.**

Give your firm a real AI assistant that can actually *act* — not a FAQ bot. Multi-agent orchestration, retrieval-augmented answers grounded in your docs, and real tool calls into your CRM, tickets, calendar, and invoices. Built as gRPC service groups — a Go data tier as the single database boundary, Python for AI orchestration and compute — deploy it behind your own website, an embeddable widget, WhatsApp, or try it in 10 minutes with the built-in Streamlit demo.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Build](https://img.shields.io/badge/build-passing-brightgreen)](.github/workflows)

</div>

---

## What this is

Most AI chatbot projects are a thin wrapper around one LLM call. ServiceSphere AI is a **backbone**: a Planner agent decomposes a request, routes it to specialist agents, which call typed tools — tools reach a Go data tier (the *only* thing that touches the database), a grouped Python compute tier, or standardized MCP servers, retrieval is grounded in your own knowledge base, and every hop is traced so you can see exactly what the assistant did and why.

Services are grouped repos that talk over **gRPC (Connect protocol)**, with `protos/` as the single contract source of truth. The core is domain-agnostic. What makes it *your* firm's assistant is a **domain pack** — a small config bundle (your services, your tone, your knowledge base, your branding) — not a fork of the code.

## Why you'd use this over building your own

- **Multi-agent, not single-prompt.** A Planner routes compound requests ("what's my ticket status, and can you also book a follow-up call") across multiple specialist agents in one turn.
- **Grounded, not hallucinated.** A real hybrid-search + re-ranking RAG pipeline over your own docs, with citations.
- **Actually does things.** Typed tool calls create tickets, book meetings, generate invoices — against your real systems, with RBAC enforced at the tool layer *and* the data tier, not just the edge.
- **One database boundary.** Only the Go `backend-services` tier holds DB credentials; every agent, compute module, and MCP server reaches data through its typed, RBAC-checked gRPC calls — a contained, auditable blast radius.
- **MCP-native.** Calendar, email, and filesystem access go through standard MCP servers, reusable by any MCP-compatible client, not a one-off integration.
- **Multi-channel from one core.** Your web app, an embeddable widget, WhatsApp, and the reference Streamlit demo all hit the same streaming chat RPC — add a channel without touching business logic.
- **Traced end to end.** Every agent step, tool call, and token is a Langfuse span — you can see exactly what happened on any given turn.
- **Open source, MIT licensed.** Self-host it, fork it, no vendor lock-in.

## Quickstart

```bash
git clone https://github.com/<your-org>/servicesphere-ai.git
cd servicesphere-ai
cp .env.example .env                                    # add your LLM provider key, or leave as-is for local Ollama
podman-compose -f infra/podman-compose.yml up -d        # mongo, redis, qdrant, minio, backend/ai/analysis services
cd frontend-services/streamlit-demo && streamlit run app.py
```

> Uses [Podman](https://podman.io) (daemonless, rootless). The compose file is Docker-compatible if you prefer `docker compose`.

Open the URL Streamlit prints — you're talking to the assistant, running on the reference IT-services domain pack, in a dark, modern UI, in a few minutes. No Next.js build, no auth setup required for the demo.

## Bring your own firm

```bash
./scripts/new-firm.sh acme-clinic
```

This scaffolds `domain-packs/acme-clinic/` from a template. Fill in:
- `business.yaml` — what services you offer, which agents/tools are enabled
- `branding.yaml` — your accent color and logo
- `system_prompt.md` — your assistant's persona
- `knowledge/` — the docs you want it grounded in

Then point the Streamlit demo, the embeddable widget, or a WhatsApp number at `tenant_id: acme-clinic`. No core code changes. See `docs/architecture/OVERVIEW.md` §21–22 for the full domain-pack and channel-adapter design.

## Supported channels

| Channel | Status |
|---|---|
| Streamlit demo | Reference implementation |
| Embeddable web widget | `channels/web-widget` |
| WhatsApp | `channels/whatsapp` |
| Next.js customer/admin portal | `frontend-services/web` |
| Slack | Planned |

## Architecture at a glance

Planner → specialist agents → typed tools → the Go data tier (`backend-services`, the only DB boundary), the grouped Python compute tier (`analysis-services`), or MCP servers — with RAG and memory grounding every turn, all over gRPC/Connect and traced in Langfuse. Full diagrams and rationale: **[`docs/architecture/OVERVIEW.md`](docs/architecture/OVERVIEW.md)**.

## Documentation map

| File | What it's for |
|---|---|
| `docs/architecture/OVERVIEW.md` | Full system design — service groups, gRPC contracts, agents, MCP servers, RAG, MongoDB, multi-tenancy |
| `PLAN.md` | Phase-by-phase build checklist (useful if you're building this from scratch, or with Claude Code) |
| `CHECKLIST.md` | Cumulative implementation tracker + version log — what's actually built and released |
| `LEARNING.md` | Module/topic curriculum map — what to learn, in what order, and where each concept's theory-and-example doc lives once generated |
| `DESIGN.md` | The shared dark/modern design system across every UI surface |
| `CLAUDE.md` | Persistent instructions for Claude Code / other coding agents working in this repo |
| `CONTRIBUTING.md` | How to add a domain pack, a channel, an MCP server, a data-access domain, or an analysis module |

## Contributing

Contributions welcome — new domain-pack examples, new channel adapters, new MCP servers, bug fixes. See [`CONTRIBUTING.md`](CONTRIBUTING.md). Please read [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) before opening a PR, and report security issues per [`SECURITY.md`](SECURITY.md) rather than as a public issue.

## License

MIT — see [`LICENSE`](LICENSE). Use it commercially, fork it, white-label it for your clients.
