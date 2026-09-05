"""Verifies the short-lived per-turn "user assertion" JWT orchestrator
mints (orchestrator/server/auth.py's mint_user_assertion) and forwards
via MCP's tools/call `_meta` field — the mechanism behind
HttpTool.auth_mode == "user_token" (protos/database/v1/http_tool.proto's
auth_mode comment has the full design).

mcp-gateway and orchestrator are both Weave-owned internal services in
the same trust zone as `core` — sharing JWT_SECRET here is the same
posture as orchestrator itself verifying core-issued access tokens with
it (orchestrator/server/auth.py). Nothing in this module is ever exposed
to a tenant; a tenant only ever sees the *output* of this verification
(a signed X-Weave-User-* header set, computed with the tenant's own
credential — see http_signing.py), never the assertion JWT or this
secret.
"""

import os

import jwt

JWT_SECRET = os.environ.get("JWT_SECRET", "")


class InvalidUserAssertionError(Exception):
    pass


def verify_user_assertion(token: str, *, expected_tenant_id: str) -> str:
    """Returns the asserted user_id if token is a valid, unexpired
    user_assertion minted for expected_tenant_id. Raises
    InvalidUserAssertionError otherwise — checked defense-in-depth
    against the tenant this MCP server instance is already scoped to
    (mcp-gateway/server.py parses tenant_id from the request path), not
    just trusting whatever tenant_id the token happens to claim."""
    if not JWT_SECRET:
        raise InvalidUserAssertionError("mcp-gateway: JWT_SECRET is not configured")
    try:
        payload = jwt.decode(token, JWT_SECRET, algorithms=["HS256"])
    except jwt.PyJWTError as exc:
        raise InvalidUserAssertionError(f"invalid or expired user assertion: {exc}") from exc

    if payload.get("typ") != "user_assertion":
        raise InvalidUserAssertionError("not a user assertion token")

    tenant_id = payload.get("tenant_id")
    user_id = payload.get("user_id")
    if not tenant_id or not user_id:
        raise InvalidUserAssertionError("user assertion missing required claims")
    if tenant_id != expected_tenant_id:
        raise InvalidUserAssertionError("user assertion was minted for a different tenant")

    return user_id
