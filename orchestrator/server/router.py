"""Supervisor routing: classifies each turn into which specialist agent
should handle it, per docs/architecture/ARCHITECTURE.md's multi-agent
design — a tools agent (the tenant's own registered connectors/HttpTools),
a web agent (public internet search, no tenant data involved), and an
analytics agent.

Honesty note: "analytics" is currently an alias for "tools", not a
distinct capability — there's no separate analytics data source in this
system yet (no metrics store, no BI connector type). Routing still
classifies it separately so the alias is a documented, deliberate
decision here, not silently hidden — and so a real analytics backend can
be plugged in later by changing only this one mapping.
"""

from llm.ollama_client import chat

Route = str  # "tools" | "web"

_CLASSIFY_SYSTEM_PROMPT = (
    "You classify a user's message into exactly one category:\n"
    "- TOOLS: the user is asking about this business's own data or services "
    "(orders, appointments, account info, anything the business itself would know).\n"
    "- WEB: the user is asking for general public information not specific to this "
    "business (current events, facts, things you'd look up online).\n\n"
    'Respond with EXACTLY one word: "TOOLS" or "WEB". No other text.'
)


async def classify_route(message: str, *, has_registered_tools: bool) -> Route:
    """Returns "tools" or "web". "analytics" is not a distinct route today
    (see module docstring) — callers that want an analytics agent should
    treat "tools" as covering it.

    Fails safe toward "tools": with no registered tools at all there's
    nothing for that route to offer anyway (the agent just answers
    directly or reports it can't help), which is a safer default than
    reaching out to the public internet on an ambiguous or failed
    classification.
    """
    if not has_registered_tools:
        return "tools"

    try:
        result = await chat(
            [
                {"role": "system", "content": _CLASSIFY_SYSTEM_PROMPT},
                {"role": "user", "content": message},
            ]
        )
    except Exception:  # noqa: BLE001 - fall back to the safe default below
        return "tools"

    verdict = result.content.strip().upper()
    if verdict.startswith("WEB"):
        return "web"
    return "tools"
