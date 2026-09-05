"""Computes the tenant-verifiable signature forwarded with a
HttpTool.auth_mode == "user_token" call — the second half of the design
in protos/database/v1/http_tool.proto's auth_mode comment (the first
half, verifying orchestrator's own assertion, is user_assertion.py).

Same well-known pattern as a webhook signing secret (Stripe/GitHub-style
inbound-request verification): the tenant registered a shared secret at
`add_tool(..., credential_secret=...)` time — same vault as any other
tool credential (docs/architecture/SECURITY.md §3) — and can verify a
request really came from Weave, on behalf of the specific user_id it
names, by recomputing the same HMAC with that same secret. Weave never
tells the tenant how to map that user_id onto their own user records —
that's the tenant's own system, same boundary every other tool call
already respects.
"""

import hashlib
import hmac


def sign_user_identity(secret: str, *, tenant_id: str, user_id: str) -> str:
    message = f"{tenant_id}:{user_id}".encode()
    return hmac.new(secret.encode(), message, hashlib.sha256).hexdigest()
