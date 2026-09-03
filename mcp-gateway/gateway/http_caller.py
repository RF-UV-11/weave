"""Executes the real HTTP call an HttpTool describes — this is the
"business logic" of mcp-gateway: turning a generic MCP tools/call into an
actual request against a business's own API.

Bugs/edge cases this deliberately handles (found by thinking through the
failure modes before writing the happy path, not after):
- Path parameters (`/orders/{order_id}/status`) — REST APIs commonly put
  identifiers in the path, not just query/body; treating every argument
  as a query/body param would break on the very first realistic example.
- Timeouts — an unbounded call to an arbitrary business endpoint could
  hang a turn forever (docs/architecture/SECURITY.md §4).
- Response size cap, enforced via streaming rather than reading the full
  body first — a truncate-after-fetch approach still lets an attacker-
  controlled endpoint make the gateway download an unbounded payload
  before the cap is applied.
- A non-2xx response is not treated as a hard failure: the body is
  returned as tool content so the model can see e.g. "404: order not
  found" and respond sensibly, rather than the tool call opaquely
  erroring out.
"""

import re

import httpx

_PATH_PARAM_RE = re.compile(r"\{(\w+)\}")
_TIMEOUT_SECONDS = 10.0
_MAX_RESPONSE_BYTES = 1_000_000  # 1 MB


class HttpToolCallError(Exception):
    """Raised for failures the caller couldn't even get a response for
    (timeout, connection refused, missing required path param) — distinct
    from a non-2xx HTTP response, which is returned as content, not raised."""


def _build_url(endpoint: str, arguments: dict) -> tuple[str, dict]:
    """Substitutes {param} placeholders in endpoint from arguments,
    returning the resolved URL and the remaining (non-path) arguments."""
    remaining = dict(arguments)

    def substitute(match: re.Match) -> str:
        key = match.group(1)
        if key not in remaining:
            raise HttpToolCallError(f"missing required path parameter {key!r}")
        return str(remaining.pop(key))

    url = _PATH_PARAM_RE.sub(substitute, endpoint)
    return url, remaining


async def call_http_tool(
    *,
    endpoint: str,
    method: str,
    arguments: dict,
    secret: str | None,
    transport: httpx.AsyncBaseTransport | None = None,
) -> str:
    url, remaining_args = _build_url(endpoint, arguments)

    headers = {}
    if secret:
        headers["Authorization"] = f"Bearer {secret}"

    method = method.upper()
    request_kwargs: dict = {"headers": headers}
    if method in ("GET", "DELETE"):
        request_kwargs["params"] = remaining_args
    else:
        request_kwargs["json"] = remaining_args

    try:
        async with httpx.AsyncClient(timeout=httpx.Timeout(_TIMEOUT_SECONDS), transport=transport) as client:
            async with client.stream(method, url, **request_kwargs) as resp:
                chunks: list[bytes] = []
                total = 0
                async for chunk in resp.aiter_bytes():
                    total += len(chunk)
                    if total > _MAX_RESPONSE_BYTES:
                        raise HttpToolCallError(f"response exceeded {_MAX_RESPONSE_BYTES} byte limit")
                    chunks.append(chunk)
                status = resp.status_code
    except httpx.TimeoutException as exc:
        raise HttpToolCallError(f"request to {url} timed out after {_TIMEOUT_SECONDS}s") from exc
    except httpx.HTTPError as exc:
        raise HttpToolCallError(f"request to {url} failed: {exc}") from exc

    body = b"".join(chunks).decode("utf-8", errors="replace")
    if status >= 400:
        return f"HTTP {status}: {body}"
    return body
