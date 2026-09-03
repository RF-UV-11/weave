"""Guardrail enforcement for external-facing bot profiles
(docs/architecture/SECURITY.md §6, BotProfile.visibility/guardrails).

Real architectural constraint this module exists because of: a "hard"
guardrail that actually prevents disclosure is fundamentally incompatible
with real-time token streaming — once a token has been sent over the
wire, it can't be unsent. So when guardrails are active for a turn, the
full answer is generated non-streamed, screened here, and only sent
(chunked, for a similar UX) once it's been approved. Profiles with no
guardrails keep genuine token-level streaming, unchanged from Phase 3.

Two checkpoints, not one: a tool's raw result is screened before it ever
enters the model's context (so a leak can't happen via "the model just
repeated what the tool told it"), and the final answer is screened again
before it's sent to the caller.
"""

from dataclasses import dataclass

from llm.ollama_client import chat

_JUDGE_SYSTEM_PROMPT = (
    "You are a strict content-safety judge. You will be given a list of rules a business has "
    "set for what its support bot must never disclose, and a piece of text the bot is about to "
    "send or has received from a tool. Decide whether the text violates any rule.\n\n"
    'Respond with EXACTLY one line: either "OK" if no rule is violated, or "VIOLATION: <short reason>" '
    "if any rule is violated. Do not add any other text."
)


@dataclass
class ScreenResult:
    ok: bool
    reason: str = ""


def _format_rules(guardrails: list[str]) -> str:
    return "\n".join(f"- {rule}" for rule in guardrails)


async def screen(text: str, guardrails: list[str]) -> ScreenResult:
    """Asks the model itself to judge text against guardrails (an
    LLM-as-judge check) — plain keyword matching is too brittle for rules
    like "never reveal supplier names," which don't reduce to a fixed
    string list. Fails closed: any judge-call error or ambiguous response
    is treated as a violation, not silently allowed through."""
    if not guardrails or not text.strip():
        return ScreenResult(ok=True)

    prompt = f"Rules:\n{_format_rules(guardrails)}\n\nText to check:\n{text}"
    try:
        result = await chat(
            [
                {"role": "system", "content": _JUDGE_SYSTEM_PROMPT},
                {"role": "user", "content": prompt},
            ]
        )
    except Exception as exc:  # noqa: BLE001 - fail closed, not open
        return ScreenResult(ok=False, reason=f"guardrail judge call failed: {exc}")

    verdict = result.content.strip()
    if verdict.upper().startswith("OK"):
        return ScreenResult(ok=True)
    if verdict.upper().startswith("VIOLATION"):
        reason = verdict.split(":", 1)[1].strip() if ":" in verdict else "guardrail violation"
        return ScreenResult(ok=False, reason=reason)

    # Model didn't follow the expected format — fail closed rather than
    # guess which way an unparseable verdict leans.
    return ScreenResult(ok=False, reason=f"unparseable judge verdict: {verdict!r}")
