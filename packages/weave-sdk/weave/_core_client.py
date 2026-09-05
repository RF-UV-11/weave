"""A thin async gRPC client wrapper around the small slice of core's
generated stubs the SDK itself needs — tenant/auth/bot_profile/http_tool/
connector, not chat/memory (those are orchestrator-only concerns; the SDK
never talks to chat).

This is a deliberately self-contained sibling of `weave_shared_clients`'
own `CoreClient` (`packages/shared-clients`), not a dependency on it: an
external integrator who does `pip install weave` (or, pre-publish, a
path/git dependency on `packages/weave-sdk`) should never need to know
`weave_shared_clients` — an internal-platform package shared by
`orchestrator`/`mcp-gateway` — exists at all, or take a second install
step to get generated gRPC bindings working. Everything the SDK needs to
talk to core ships inside this package.

Generated code lives in ./gen (produced by protoc/grpc_tools at dev-setup
time — see initialize.sh — not committed, same convention as every other
gen/ in this repo) and uses protoc's default non-package-relative imports
(e.g. `from core.data_access.v1 import tenant_pb2`), so `gen/` has to be
importable as a set of top-level packages. This module puts it on
sys.path before importing anything generated, exactly like
weave_shared_clients/client.py does — same proven mechanism, just scoped
to this package's own bundled stubs instead of a sibling one.
"""

import sys
from pathlib import Path

_GEN_DIR = Path(__file__).parent / "gen"
if str(_GEN_DIR) not in sys.path:
    sys.path.insert(0, str(_GEN_DIR))

import grpc  # noqa: E402

from core.data_access.v1 import auth_pb2_grpc  # noqa: E402
from core.data_access.v1 import bot_profile_pb2_grpc  # noqa: E402
from core.data_access.v1 import connector_pb2_grpc  # noqa: E402
from core.data_access.v1 import http_tool_pb2_grpc  # noqa: E402
from core.data_access.v1 import tenant_pb2_grpc  # noqa: E402


def bearer_metadata(token: str) -> list[tuple[str, str]]:
    """gRPC call metadata carrying a JWT — the same "authorization: Bearer
    <token>" shape core's shared-auth interceptor expects."""
    return [("authorization", f"Bearer {token}")]


class CoreClient:
    """Owns one async channel to core and exposes a stub per service this
    SDK actually calls. Tenant/Connector RPCs don't require a token yet
    (docs/architecture/SECURITY.md's known gap); Auth/BotProfile/HttpTool
    do — pass bearer_metadata(token) as the `metadata` kwarg on those.
    """

    def __init__(self, target: str):
        self.channel = grpc.aio.insecure_channel(target)
        self.tenant = tenant_pb2_grpc.TenantServiceStub(self.channel)
        self.connector = connector_pb2_grpc.ConnectorServiceStub(self.channel)
        self.auth = auth_pb2_grpc.AuthServiceStub(self.channel)
        self.bot_profile = bot_profile_pb2_grpc.BotProfileServiceStub(self.channel)
        self.http_tool = http_tool_pb2_grpc.HttpToolServiceStub(self.channel)

    async def close(self) -> None:
        await self.channel.close()

    async def __aenter__(self) -> "CoreClient":
        return self

    async def __aexit__(self, *exc: object) -> None:
        await self.close()
