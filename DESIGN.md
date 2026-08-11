# Weave — Design System

Applies to every UI surface: the onboarding dashboard, admin console, chat UI, and the embeddable widget. Dark-mode-first — there is no light-only screen anywhere in the product.

---

## 1. Concept

Weave's visual identity is built around the product's core metaphor: **two threads interlacing** — the tenant's own systems (warm thread) and Weave's reasoning (cool thread) — crossing at each step of a request. The signature UI element, **the Weave Line**, is an animated trace showing a message's actual path: `Channel → Planner → Agent → Connector → Response`, rendered as two colored threads that meet at each hop. This isn't decoration — it's the same trust-and-traceability idea from `docs/architecture/SECURITY.md` §7 made visible: a user can see which connector actually answered.

## 2. Color tokens

Dark-mode-first. Values below are the default (unbranded) palette; a tenant can override the accent pair via their branding config (§6), never the neutrals.

| Token | Value | Use |
|---|---|---|
| `--bg` | `#0B0D14` | App background |
| `--surface` | `#12151F` | Cards, panels, chat bubbles |
| `--surface-raised` | `#1A1E2B` | Modals, popovers |
| `--border` | `#242938` | Dividers, card borders |
| `--text-primary` | `#E6E8F0` | Primary text |
| `--text-secondary` | `#9AA0B4` | Secondary/meta text |
| `--thread-cool` (accent A — Weave's reasoning) | `#7B6CFF` | Planner/agent steps, primary actions, links |
| `--thread-warm` (accent B — the tenant's system) | `#FFB86B` | Connector/tool-call steps, "acting on your system" states |
| `--success` | `#3DDC97` | Healthy connector, completed action |
| `--warning` | `#F5B942` | Degraded connector, needs attention |
| `--danger` | `#FF6B6B` | Failed tool call, revoked credential |

A light theme is not planned; if one is ever added, it inverts neutrals only and keeps both thread colors at the same hue, adjusted for contrast.

## 3. Typography

| Role | Font | Notes |
|---|---|---|
| Display (headings, marketing) | Space Grotesk | Geometric, distinct at large sizes |
| Body / UI | Inter | High legibility at small sizes, wide weight range |
| Data / code / connector IDs | IBM Plex Mono | Anything that's an identifier, a schema, or a trace value |

## 4. The Weave Line (signature component)

- Renders as a horizontal (desktop) or vertical (mobile) trace beneath each assistant turn: `Channel → Planner → Agent → Connector → Response`.
- Each segment animates in order as the actual request progresses (not a fake loading bar) — driven by real intermediate stream events from the chat API, matching the request lifecycle in `docs/architecture/ARCHITECTURE.md` §2.
- Segments touching a tenant connector render in `--thread-warm`; segments internal to Weave's own reasoning render in `--thread-cool`. This color split is deliberate: a user should always be able to tell, at a glance, when the assistant is reaching into *their* system versus reasoning on its own.
- A connector segment is clickable/expandable to show which connector served it and its latency — direct UI expression of the auditability principle in `SECURITY.md` §7.

## 5. Component rules

- **Chat window**: streamed tokens render immediately; tool-call cards show pending → running → done, each tagged with its connector name when applicable ("Checking Acme Booking…", not a generic "Working…").
- **Connector cards** (dashboard): status pill (`healthy` / `degraded` / `unreachable`, using the semantic tokens above), last manifest refresh time, and which bot profiles use it.
- **Bot profile switcher**: visually distinct per profile (a small color tag, not a full theme change) so an admin never confuses which profile they're editing or testing.
- **Empty/zero states**: every "no connectors yet" / "no bot profile configured" state includes the next concrete action, never just an illustration.

## 6. White-labeling

A tenant can override, per `packs/<tenant>/branding.yaml`:
```yaml
accent_cool: "#7B6CFF"   # optional override of --thread-cool
accent_warm: "#FFB86B"   # optional override of --thread-warm
logo_url: "https://…"
display_font_override: null   # optional, falls back to Space Grotesk
```
Neutrals (`--bg`, `--surface`, `--border`, text tokens) are **not** overridable — this keeps every tenant's assistant reading as unmistakably a Weave product even when white-labeled, the same way the Weave Line's two-thread behavior stays structurally identical regardless of the tenant's chosen colors.

## 7. Accessibility

- Minimum contrast ratio 4.5:1 for body text against `--bg`/`--surface` at default token values — verify again if a tenant overrides accent colors used for text (not just accents used for non-text UI).
- The Weave Line's animation respects `prefers-reduced-motion` — falls back to a static, fully-drawn trace.
- Every connector/tool-call status is conveyed by icon + text, never color alone.
