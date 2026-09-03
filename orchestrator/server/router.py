"""Supervisor routing: classifies each turn into which specialist agent
should handle it, per docs/architecture/ARCHITECTURE.md's multi-agent
design — a tools agent (the tenant's own registered connectors/HttpTools),
a web agent (public internet search, no tenant data involved, vendor-
gated by BotProfile.web_search_enabled), and an analytics agent (the
tenant's own HttpTools tagged category="analytics" via HttpTool.category
— see mcp-gateway/gateway/tenant_server.py's _meta and
mcp_client/client.py's McpTool.category).

Each route is only ever offered to the classifier if it's actually
available this turn (has_analytics_tools / web_search_enabled) — there's
no point letting the model pick a route with nothing behind it, and
offering an unavailable route just adds a class of misclassification to
fail-safe around.
"""

from llm.ollama_client import chat

Route = str  # "tools" | "web" | "analytics"

_BASE_CATEGORIES = (
    "- TOOLS: the user is asking about this business's own data or services "
    "(orders, appointments, account info, anything the business itself would know).\n"
)
_ANALYTICS_CATEGORY = (
    "- ANALYTICS: the user is asking for aggregate/statistical insight into this "
    "business's own data (revenue, sales trends, counts over time, reports) rather "
    "than a single record.\n"
)
_WEB_CATEGORY = (
    "- WEB: the user is asking for general public information not specific to this "
    "business (current events, facts, things you'd look up online).\n"
)


async def classify_route(
    message: str,
    *,
    has_registered_tools: bool,
    has_analytics_tools: bool = False,
    web_search_enabled: bool = False,
) -> Route:
    """Returns "tools", "analytics", or "web" — only ever a route that's
    actually available this turn (see module docstring).

    Fails safe toward "tools": with no registered tools and no web search
    there's nothing for any other route to offer anyway (the agent just
    answers directly or reports it can't help), which is a safer default
    than reaching out to the public internet on an ambiguous or failed
    classification.
    """
    if not has_registered_tools and not web_search_enabled:
        return "tools"

    if not has_analytics_tools and not web_search_enabled:
        # Only one real route besides "tools" would be offered and it
        # isn't available either — skip the LLM call entirely.
        return "tools"

    categories = _BASE_CATEGORIES
    valid = {"TOOLS"}
    if has_analytics_tools:
        categories += _ANALYTICS_CATEGORY
        valid.add("ANALYTICS")
    if web_search_enabled:
        categories += _WEB_CATEGORY
        valid.add("WEB")

    prompt = (
        "You classify a user's message into exactly one category:\n"
        f"{categories}\n"
        f'Respond with EXACTLY one word: {" or ".join(sorted(valid))}. No other text.'
    )

    try:
        result = await chat(
            [
                {"role": "system", "content": prompt},
                {"role": "user", "content": message},
            ]
        )
    except Exception:  # noqa: BLE001 - fall back to the safe default below
        return "tools"

    verdict = result.content.strip().upper()
    for route_word in valid:
        if verdict.startswith(route_word):
            return route_word.lower()
    return "tools"
