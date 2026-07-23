# DESIGN.md — Visual Identity & Design System

Applies to **every UI surface** in this repo: the Next.js customer/admin portal, the Streamlit demo chatbot, the embeddable web widget, and any future channel UI. One system, several renderers.

The product's real subject is a **request tracing a path through a system** — a message enters, a Planner routes it, agents fire, tools/MCP servers get called, an answer streams back. The design should make that path visible rather than hiding it behind a generic chat bubble. That idea — not "dark mode with an accent color" — is the actual design brief.

Avoid the two default AI-tool looks (warm cream + terracotta serif; near-black + acid-green/vermilion). This system is dark, but graphite/blue-shifted rather than near-black, and its signature element is functional, not decorative.

---

## 1. Token system

**Color** (dark-mode-first; this *is* the default theme, not a toggle):

| Token | Hex | Use |
|---|---|---|
| `canvas` | `#0B0E14` | App background — graphite, slightly blue-shifted, never pure black |
| `surface` | `#12161F` | Cards, panels, message bubbles |
| `surface-raised` | `#1A2030` | Modals, dropdowns, the trace strip |
| `border` | `#262D3D` | Hairline dividers — used instead of drop shadows for separation |
| `text-primary` | `#E7E9EE` | Body text, headings |
| `text-secondary` | `#8B93A7` | Timestamps, labels, metadata |
| `signal` (accent) | `#5B8CFF` | The one hero color — active states, streaming cursor, the trace pulse, primary buttons |
| `confirm` | `#34D399` | Success states only (ticket created, meeting booked) — used sparingly, never as decoration |
| `warn` | `#F5A524` | Warnings, SLA-approaching states |
| `error` | `#F2545B` | Errors, failed tool calls |

**Typography** (three roles, none of them the generic Inter-everywhere default):

- **Display** — Space Grotesk. Geometric, slightly technical. Used restrained: page titles, section headers, the landing page hero. Never body copy.
- **Body** — IBM Plex Sans (or Inter if a team prefers a safer fallback). Clean, highly legible at small sizes for chat text.
- **Mono/utility** — IBM Plex Mono or JetBrains Mono. Used for anything that *is* data: trace IDs, tool-call payloads, timestamps, session IDs, code blocks. This is what makes tool-call cards read as "real system output" rather than decoration.

**Shape & elevation**
- Radius: 8–10px. Small and precise, not fully rounded — the product is infrastructure, not a consumer social app.
- No heavy drop shadows. Elevation is communicated by `surface` → `surface-raised` shade shift plus a 1px `border` hairline.
- Generous negative space over dense panels; this is a chat-first product, not a data-dense admin grid (analytics screens are the one exception — see §4).

**Motion**
- One deliberate animation: the trace pulse (§2). Everything else — hovers, panel opens, tab switches — is quick and quiet (150–200ms ease-out), and respects `prefers-reduced-motion`.
- Don't add ambient background animation, gradient shifts, or decorative particle effects. Spend the one allowed moment of boldness on the trace.

---

## 2. Signature element: the Live Signal Path

When a message triggers tool calls or multi-agent routing, render a horizontal step trace above the assistant's reply:

```
●─────●─────●─────●
Planner  Agent  Tool  Reply
```

- Each node lights up in `signal` color as that step starts, holds while running (subtle pulse), and settles to a solid dot when done.
- The connecting line between nodes animates a traveling pulse of `signal` color while that hop is in-flight — this is the one animation in the whole system, and it's literally showing the architecture (Planner → Agent → Tool/MCP → Response) at work.
- After completion, the trace collapses to a thin, click-to-expand strip (node labels + timing, e.g. "Ticket Agent · 340ms") so power users can inspect it like a mini Langfuse trace, without cluttering the chat for everyone else.
- Reuse this exact motif in: the Next.js chat UI, the Streamlit demo (simplified — see §5), and any future channel that supports rich rendering. Channels that can't render it (plain WhatsApp text) get a plain-text fallback: `⚙️ Checked ticket status → 🔧 Booked meeting`.

This element is the thing a firm's engineering team should recognize the product by — not a logo, this.

---

## 3. Chat surface

- User messages: right-aligned, plain text on `surface`, no bubble outline needed beyond the background shade.
- Assistant messages: left-aligned, `surface` background, `border` hairline, trace strip (§2) above the text when tools fired, citation chips (small, `text-secondary`, mono) inline when RAG sources were used.
- Streaming: a `signal`-colored blinking cursor block at the end of in-flight text, not a generic spinner.
- Tool-call cards (styled fully in `PLAN.md` Phase 12, the Next.js frontend; approximated earlier in the Phase 2 Streamlit dev tool) are just the trace strip's expanded node — don't build a second, differently-styled component for the same concept.

---

## 4. Dashboards (Next.js admin/customer portal)

- Same token system, but density goes up: tables and charts (recharts) use `surface`/`surface-raised`/`border` for structure, `signal`/`confirm`/`warn`/`error` only for status — never decorative chart colors outside that palette.
- Empty states and errors follow the writing rules in §6 — plain, specific, in the interface's voice.

---

## 5. Streamlit demo chatbot

Streamlit can't fully render the animated trace, so approximate it, don't abandon it:

- `.streamlit/config.toml`:
  ```toml
  [theme]
  base = "dark"
  primaryColor = "#5B8CFF"
  backgroundColor = "#0B0E14"
  secondaryBackgroundColor = "#12161F"
  textColor = "#E7E9EE"
  font = "sans serif"
  ```
- Inject the display/mono fonts and hide Streamlit's default chrome (hamburger menu, "Made with Streamlit" footer, default padding) via one shared `frontend-services/streamlit-demo/theme.py` that returns a `<style>` block used at the top of `app.py`.
- Render the trace as a static row of `st.columns` with colored circle emoji/markup lighting up in sequence as steps complete (`st.status()` / `st.empty()` placeholders updated during the streamed response) — same node labels and order as the Next.js version, just without the traveling-pulse animation.
- This file is the reference implementation of the whole design system in its simplest form — if a design token doesn't survive translation into Streamlit, reconsider whether it belongs in the system at all.

---

## 6. Writing rules (apply everywhere)

- Name things by what the user controls, not how the system is built: "Turn on email replies," not "Configure notification webhook."
- Buttons/actions keep the same name through the whole flow: a "Create ticket" button produces a "Ticket created" confirmation, not "Success."
- Errors state what happened and how to fix it, in the product's voice — never "Oops!" or an apology.
- Empty states are an invitation to act ("No tickets yet — ask the assistant to open one"), not a dead end.

---

## 7. Applying this to a firm's own brand (white-label)

A firm using this as their backbone overrides, per `domain-packs/<firm>/branding.yaml`:
- `signal` accent color (their brand color) — everything else in the token table stays fixed so the system still *reads* as this product's design language underneath their color.
- Logo + display font swap (optional, falls back to Space Grotesk).
- Never let a firm override `canvas`/`surface` to light mode as a per-tenant toggle without deliberately re-deriving the whole token table — a naive "invert colors" light mode breaks the elevation and trace-pulse legibility. If a firm insists on light mode, that's a real design pass against this same system, not a CSS variable flip.
