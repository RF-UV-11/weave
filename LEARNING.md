# LEARNING.md — Curriculum Index & Documentation Generator Map

This is the single map of **everything this project teaches**, organized module → topic, each topic tied to a real place in `PLAN.md`'s build phases and to an exact file path where its learning document belongs.

**The rule this file enforces**: when a `PLAN.md` task tied to a topic below is completed, the corresponding doc at the listed path gets generated (or updated, if it already exists) — in full, following the template in §1, not as a stub. This file itself is the index; the actual theory-and-example content lives in the generated `docs/roadmap/**` files it points to. `CLAUDE.md`'s "Working style" wires this into the normal build workflow so it happens automatically as the project gets built, not as a separate pass at the end.

---

## 1. The doc generation contract

Every generated topic doc, regardless of module, follows this exact structure. Copy this skeleton to start a new one:

```markdown
# <Topic Title>

> Module <N> · Phase <PLAN.md phase> · Generated from: <the actual file(s)/PR this doc documents>

## 1. Theory
Plain-language explanation of the concept: what problem it solves, why it exists, how it fits
into the bigger system this project is building. Assume the reader knows general programming
but not this specific concept.

## 2. Key Concepts & Terminology
- **Term** — one-line definition, specific to how it's used here (not a generic glossary copy).

## 3. How this project uses it
Point at the real files: "This is implemented in `ai-services/tools/ticket_tools.py`."
Explain the actual design decision made here, and why — not just the generic version of the concept.

## 4. Worked example
A real, runnable code example. Prefer lifting the actual pattern used in this repo over inventing
a toy example — the reader should be able to open the real file right after reading this section
and recognize it.

## 5. Diagram (where it clarifies more than prose)
Mermaid flowchart/sequence diagram if the concept is structural or has a request flow.

## 6. Exercise
1–3 hands-on exercises that extend or modify the real repo code (not a disconnected toy problem),
e.g. "Add a second tool following this pattern" or "Trace what changes if X."

## 7. Common mistakes
The 2–4 mistakes someone actually makes with this concept, and why they're wrong — pulled from
real failure modes, not generic caution.

## 8. Further reading
1–3 external links (official docs, papers) for going deeper than this project needs to.

## 9. Related topics
Links to other docs in this curriculum that build on or connect to this one.
```

A doc that's missing worked examples grounded in this repo's actual code, or that reads as generic theory copy-pasted from anywhere, isn't done — regenerate it.

---

## 2. Module → Topic → Doc Path map

Status: `[ ]` not yet written · `[~]` stub exists, needs the full template · `[x]` complete per §1.

### Module 0 — Contracts & Data Tier *(Phase 0)*
| Topic | Doc path | Status |
|---|---|---|
| Protocol Buffers & `buf` codegen | `docs/roadmap/00-contracts-data/01-protobuf-buf.md` | [ ] |
| gRPC & Connect (transport, streaming, gRPC-Web) | `docs/roadmap/00-contracts-data/02-grpc-connect.md` | [ ] |
| Go data-access services (`connect-go`) | `docs/roadmap/00-contracts-data/03-go-data-access.md` | [ ] |
| MongoDB modeling & the single-DB-tier boundary | `docs/roadmap/00-contracts-data/04-mongodb-data-tier.md` | [ ] |

### Module 1 — LLM & Prompting Foundations *(Phase 2)*
| Topic | Doc path | Status |
|---|---|---|
| LLM fundamentals (tokens, context window, temperature, sampling) | `docs/roadmap/01-llm-foundations/01-llm-fundamentals.md` | [ ] |
| Prompt engineering (system/user roles, few-shot, instructions) | `docs/roadmap/01-llm-foundations/02-prompt-engineering.md` | [ ] |
| Structured output (Pydantic schemas, JSON mode) | `docs/roadmap/01-llm-foundations/03-structured-output.md` | [ ] |
| Streaming responses (Connect server-streaming, token-by-token) | `docs/roadmap/01-llm-foundations/04-streaming-responses.md` | [ ] |

### Module 2 — Tool & Function Calling *(Phase 2)*
<!-- Modules 1 and 2 both land in Phase 2 ("AI Services Core") now — the old
     Basic Chat / Tool Calling phases merged when the frontend moved out. -->

| Topic | Doc path | Status |
|---|---|---|
| Function calling fundamentals | `docs/roadmap/02-tool-calling/01-function-calling.md` | [ ] |
| Tool calling as a trust boundary | `docs/roadmap/02-tool-calling/02-tool-calling-architecture.md` | [ ] |
| Tool → `backend-services` RPC (the data boundary) | `docs/roadmap/02-tool-calling/03-tool-to-data-rpc.md` | [ ] |
| External API calling (weather/currency/news, routing decision) | `docs/roadmap/02-tool-calling/04-external-api-calling.md` | [ ] |

### Module 3 — Retrieval-Augmented Generation *(Phase 4)*
| Topic | Doc path | Status |
|---|---|---|
| Embeddings | `docs/roadmap/03-rag/01-embeddings.md` | [ ] |
| Vector databases (Qdrant/Chroma) | `docs/roadmap/03-rag/02-vector-databases.md` | [ ] |
| Chunking & metadata design | `docs/roadmap/03-rag/03-chunking-metadata.md` | [ ] |
| Hybrid search (dense + sparse) | `docs/roadmap/03-rag/04-hybrid-search.md` | [ ] |
| Re-ranking | `docs/roadmap/03-rag/05-reranking.md` | [ ] |
| RAG evaluation (recall@k, faithfulness) | `docs/roadmap/03-rag/06-rag-evaluation.md` | [ ] |

### Module 4 — Memory & Conversation State *(Phase 5)*
| Topic | Doc path | Status |
|---|---|---|
| Conversation state management | `docs/roadmap/04-memory/01-conversation-state.md` | [ ] |
| Short-term (working) memory | `docs/roadmap/04-memory/02-short-term-memory.md` | [ ] |
| Long-term & semantic memory | `docs/roadmap/04-memory/03-long-term-semantic-memory.md` | [ ] |
| User preferences | `docs/roadmap/04-memory/04-user-preferences.md` | [ ] |

### Module 5 — Model Context Protocol (MCP) *(Phase 8)*
| Topic | Doc path | Status |
|---|---|---|
| MCP protocol fundamentals (initialize, transports) | `docs/roadmap/05-mcp/01-mcp-protocol.md` | [ ] |
| Building an MCP server | `docs/roadmap/05-mcp/02-mcp-server.md` | [ ] |
| Building an MCP client | `docs/roadmap/05-mcp/03-mcp-client.md` | [ ] |
| MCP resources, tools & prompts | `docs/roadmap/05-mcp/04-resources-tools-prompts.md` | [ ] |

### Module 6 — Multi-Agent Orchestration *(Phase 6)*
| Topic | Doc path | Status |
|---|---|---|
| Multi-agent system design | `docs/roadmap/06-multi-agent/01-multi-agent-design.md` | [ ] |
| LangGraph fundamentals (StateGraph, edges, checkpointing) | `docs/roadmap/06-multi-agent/02-langgraph-fundamentals.md` | [ ] |
| Planner/router pattern | `docs/roadmap/06-multi-agent/03-planner-router-pattern.md` | [ ] |
| AI SDK comparison: LangGraph vs. Pydantic AI | `docs/roadmap/06-multi-agent/04-langgraph-vs-pydantic-ai.md` | [ ] |

### Module 7 — Grouped Compute (`analysis-services`) *(Phase 7)*
| Topic | Doc path | Status |
|---|---|---|
| Grouped-service pattern (one server, modules via routes) | `docs/roadmap/07-grouped-compute/01-grouped-service-pattern.md` | [ ] |
| Modular monolith vs. microservice: when to group | `docs/roadmap/07-grouped-compute/02-when-to-group.md` | [ ] |
| Compute modules that call back for data | `docs/roadmap/07-grouped-compute/03-compute-calling-data.md` | [ ] |

### Module 8 — Production Contracts & Auth *(Phase 10)*
<!-- Auth first appears in Phase 1 (backend core domains); this module's docs
     are best written once RBAC is wired everywhere and re-checked at the tool
     layer too, which is Phase 10 ("Backend Completion"). -->

| Topic | Doc path | Status |
|---|---|---|
| Service-group architecture & the data trust boundary | `docs/roadmap/08-production-contracts/01-service-group-architecture.md` | [ ] |
| Authentication (JWT in Connect metadata) | `docs/roadmap/08-production-contracts/02-authentication-jwt.md` | [ ] |
| Authorization (RBAC interceptor, re-checked at the tool layer) | `docs/roadmap/08-production-contracts/03-authorization-rbac.md` | [ ] |
| Contract standards (proto versioning, pagination, idempotency) | `docs/roadmap/08-production-contracts/04-contract-standards.md` | [ ] |

### Module 9 — Observability & Evaluation *(Phase 9)*
| Topic | Doc path | Status |
|---|---|---|
| LLM/agent tracing (Langfuse) | `docs/roadmap/09-observability/01-llm-tracing-langfuse.md` | [ ] |
| Infra monitoring (OpenTelemetry across Go + Python) | `docs/roadmap/09-observability/02-opentelemetry.md` | [ ] |
| Offline & online evaluation | `docs/roadmap/09-observability/03-evaluation.md` | [ ] |

### Module 10 — Deployment & Infrastructure *(Phase 13)*
| Topic | Doc path | Status |
|---|---|---|
| Podman & podman-compose (Containerfiles, rootless) | `docs/roadmap/10-deployment/01-podman-compose.md` | [ ] |
| Kubernetes (Deployments, HPA, Kustomize, `podman generate kube`) | `docs/roadmap/10-deployment/02-kubernetes.md` | [ ] |
| CI/CD pipelines (buf lint/breaking, go test, pytest, image build) | `docs/roadmap/10-deployment/03-ci-cd.md` | [ ] |

### Module 11 — Generalization: Domain Packs, Multi-Tenancy & Channels *(Phases 3, 11–13)*
<!-- Channels now build in Phase 3 (right after ai-services core) rather than
     late — the "channel adapter pattern" topic can be written as soon as
     Phase 3 is done, even though domain packs/multi-tenancy (Phase 11),
     design systems (Phase 12, the frontend), and OSS structure (Phase 13)
     land later. Topics didn't move modules, only the phases they're tied to. -->
| Topic | Doc path | Status |
|---|---|---|
| Config-driven domain design | `docs/roadmap/11-generalization/01-domain-pack-design.md` | [ ] |
| Multi-tenancy architecture | `docs/roadmap/11-generalization/02-multi-tenancy.md` | [ ] |
| Channel adapter pattern | `docs/roadmap/11-generalization/03-channel-adapters.md` | [ ] |
| Design systems for AI products | `docs/roadmap/11-generalization/04-design-systems.md` | [ ] |
| Open-source project structure | `docs/roadmap/11-generalization/05-open-source-structure.md` | [ ] |

---

## 3. How this ties into `PLAN.md`

Each `PLAN.md` phase lists concrete build tasks. This file adds one more implicit task to every phase: **before checking off the phase's "Definition of done," generate/update every topic doc in that phase's module above.** Practically:

1. Finish the phase's build tasks in `PLAN.md` as normal.
2. Open this file, find the module matching that phase.
3. For each topic, write (or update) the doc at its path using the §1 template, grounded in the code you just built.
4. Flip its status to `[x]` here.
5. Only then check the phase's "Definition of done" box in `PLAN.md` (and update `CHECKLIST.md`).

If a phase's build reveals a topic isn't covered above (e.g. you hit a real concept mid-build that doesn't map cleanly to an existing row), add a row to the relevant module rather than skipping it — this table should stay a complete map of what the finished project actually teaches.

## 4. Reading order (if learning end-to-end rather than building)

Modules 0 → 11 in order is the intended *conceptual* reading path — each module still assumes the previous ones, even though `PLAN.md`'s build phases no longer map to modules 1:1 in strict ascending order (building is module-by-module by service group, with `frontend-services/web` deliberately last; learning stays concept-by-concept). Module 0 (contracts + the Go/Mongo data tier) comes first deliberately: every later module reaches data through it, so the boundary has to be understood before the AI layers that depend on it. Within a module, read topics top to bottom. `docs/architecture/OVERVIEW.md` is the reference for *system design* at any point; this file plus `docs/roadmap/**` is the reference for *learning the underlying concepts* — read OVERVIEW.md's relevant section first for context, then the matching topic doc here for depth.
