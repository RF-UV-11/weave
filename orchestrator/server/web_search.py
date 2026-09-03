"""A built-in web-search tool for the "web agent" route
(docs/architecture/ARCHITECTURE.md's multi-agent supervisor) — unlike
every other tool in this system, this one isn't registered by a tenant
(core.HttpTool) or backed by a real MCP connector; it's a standing
capability orchestrator offers directly, no API key required.

Uses DuckDuckGo's HTML-only endpoint (no API key, no rate-limit
headaches from a paid search API) via light regex extraction rather than
a full HTML parser — good enough for a first cut, and avoids adding a new
dependency (beautifulsoup4) for what's currently one page's worth of
scraping. If DuckDuckGo's markup changes, this degrades to "no results,"
not a crash — see the try/except around parsing.
"""

import html
import re

import httpx

from mcp_client.client import McpTool
from tools.assembly import AssembledTool

_SEARCH_URL = "https://html.duckduckgo.com/html/"
_TIMEOUT_SECONDS = 10.0
_MAX_RESULTS = 5

# DuckDuckGo's HTML-lite result markup: an <a class="result__a" href="...">
# title</a> per result, with a following result__snippet block. Matched
# with re.DOTALL since the snippet can span the anchor tag's own newlines.
_RESULT_RE = re.compile(
    r'<a[^>]*class="result__a"[^>]*href="(?P<url>[^"]+)"[^>]*>(?P<title>.*?)</a>.*?'
    r'class="result__snippet"[^>]*>(?P<snippet>.*?)</a>',
    re.DOTALL,
)


def _strip_tags(raw: str) -> str:
    return html.unescape(re.sub(r"<[^>]+>", "", raw)).strip()


async def run_web_search(query: str, *, transport: httpx.AsyncBaseTransport | None = None) -> str:
    async with httpx.AsyncClient(timeout=httpx.Timeout(_TIMEOUT_SECONDS), transport=transport) as client:
        resp = await client.post(_SEARCH_URL, data={"q": query}, headers={"User-Agent": "Mozilla/5.0"})
        resp.raise_for_status()

    matches = _RESULT_RE.finditer(resp.text)
    results = []
    for m in matches:
        if len(results) >= _MAX_RESULTS:
            break
        title = _strip_tags(m.group("title"))
        snippet = _strip_tags(m.group("snippet"))
        if title:
            results.append(f"{title}\n{snippet}")

    if not results:
        return f"No web results found for {query!r}."
    return "\n\n".join(results)


WEB_SEARCH_TOOL = AssembledTool(
    connector_id="built-in",
    connector_name="web",
    endpoint="",
    tool=McpTool(
        name="web_search",
        description="Search the public web for current information not available from the business's own tools.",
        input_schema={
            "type": "object",
            "properties": {"query": {"type": "string", "description": "The search query."}},
            "required": ["query"],
        },
    ),
)


def is_web_search(tool_name: str) -> bool:
    return tool_name == WEB_SEARCH_TOOL.tool.name
