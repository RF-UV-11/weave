"""Verifies the same JWTs core issues (packages/shared-auth, Go) —
orchestrator has its own minimal verifier here rather than depending on
a Go package from Python. Same HS256/claims shape: {tenant_id, user_id,
role, typ, iat, exp}. Kept intentionally tiny; if a second Python service
ever needs this, promote it into a shared package then, not before.
"""

import os
from dataclasses import dataclass

import jwt

JWT_SECRET = os.environ.get("JWT_SECRET", "")


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
