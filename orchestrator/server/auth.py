"""Verifies the same JWTs core issues (packages/shared-auth, Go) —
orchestrator has its own minimal verifier here rather than depending on
a Go package from Python. Same HS256/claims shape: {tenant_id, user_id,
role, typ, iat, exp}. Kept intentionally tiny; if a second Python service
ever needs this, promote it into a shared package then, not before.

Also mints a second, narrower kind of token — a "user assertion"
(mint_user_assertion) — used for HttpTool.auth_mode == "user_token"
(docs/architecture/ARCHITECTURE.md §3, protos/database/v1/http_tool.proto's
auth_mode comment): a short-lived {tenant_id, user_id} JWT orchestrator
attaches to a tools/call request so mcp-gateway can prove to a tenant's
own endpoint which specific signed-in user is asking, without forwarding
the caller's real access token (narrower scope, shorter lifetime, and a
distinct `typ` so it can never be replayed as a real access token).
Signed with the same JWT_SECRET core/orchestrator already share — this
assertion never leaves Weave's own trust zone until mcp-gateway (also a
Weave-owned service) turns it into a tenant-specific HMAC signature; the
tenant never sees this JWT or JWT_SECRET itself.
"""

import os
import time
from dataclasses import dataclass

import jwt

JWT_SECRET = os.environ.get("JWT_SECRET", "")

# Deliberately short — this only ever needs to live for the duration of
# one turn's tool calls, never persisted or replayed across turns.
_USER_ASSERTION_TTL_SECONDS = 60


class InvalidTokenError(Exception):
    pass


@dataclass
class Claims:
    tenant_id: str
    user_id: str
    role: str


def verify_access_token(token: str) -> Claims:
    if not JWT_SECRET:
        raise InvalidTokenError("orchestrator: JWT_SECRET is not configured")
    try:
        payload = jwt.decode(token, JWT_SECRET, algorithms=["HS256"])
    except jwt.PyJWTError as exc:
        raise InvalidTokenError(f"invalid or expired token: {exc}") from exc

    if payload.get("typ") != "access":
        raise InvalidTokenError("not an access token")

    tenant_id = payload.get("tenant_id")
    user_id = payload.get("user_id")
    role = payload.get("role")
    if not tenant_id or not user_id or not role:
        raise InvalidTokenError("token missing required claims")

    return Claims(tenant_id=tenant_id, user_id=user_id, role=role)


def bearer_token_from_metadata(metadata: tuple[tuple[str, str], ...]) -> str:
    for key, value in metadata or ():
        if key.lower() == "authorization" and value.startswith("Bearer "):
            return value[len("Bearer ") :]
    raise InvalidTokenError("missing authorization metadata")


def mint_user_assertion(tenant_id: str, user_id: str) -> str:
    """Signs a short-lived {tenant_id, user_id, typ: "user_assertion"} JWT
    — see this module's docstring. Minted once per ChatStream turn
    (server/chat_service.py) and carried through tools/call's MCP _meta
    field (mcp_client/client.py) for every tool call in that turn,
    whether or not the target tool actually needs it — mcp-gateway is
    what decides that, per-tool, based on HttpTool.auth_mode."""
    if not JWT_SECRET:
        raise InvalidTokenError("orchestrator: JWT_SECRET is not configured")
    now = int(time.time())
    payload = {
        "typ": "user_assertion",
        "tenant_id": tenant_id,
        "user_id": user_id,
        "iat": now,
        "exp": now + _USER_ASSERTION_TTL_SECONDS,
    }
    return jwt.encode(payload, JWT_SECRET, algorithm="HS256")
