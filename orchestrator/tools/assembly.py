"""Dynamic tool assembly (docs/architecture/ARCHITECTURE.md §3): per
request, resolve the active bot profile for this tenant+channel, fetch
its registered connectors from core, and discover each connector's tools
live over MCP — never a hardcoded tool catalog.

Filter applied here, matching the architecture doc's diagram exactly:
role ∈ profile.roles_allowed AND (the connector set is already scoped to
the profile's connector_ids by construction).
"""

from dataclasses import dataclass

from weave_shared_clients import CoreClient, bearer_metadata

# weave_shared_clients puts its gen/ directory on sys.path as a side
# effect of import, which is what makes these resolvable.
from core.data_access.v1 import bot_profile_pb2, connector_pb2

from mcp_client import McpTool, list_tools

# Matches database.v1.Role's enum values (protos/database/v1/auth.proto) —
# kept as plain strings here so tool-assembly logic doesn't need to import
# the generated proto enum just to compare against the JWT's string role
# claim (core/rpc_services/auth/role.go does the same enum<->string
# mapping on the Go side).
ROLE_ENUM_TO_NAME = {0: "", 1: "owner", 2: "admin", 3: "staff", 4: "customer"}


class ToolAssemblyError(Exception):
    """Raised when tool assembly can't proceed — no active bot profile
    for this channel, or the caller's role isn't allowed to use it."""


@dataclass
class AssembledTool:
    connector_id: str
    connector_name: str
    endpoint: str
    tool: McpTool


async def assemble_tools(
    core: CoreClient,
    *,
    tenant_id: str,
    channel: str,
    role: str,
    token: str,
) -> list[AssembledTool]:
    metadata = bearer_metadata(token)

    try:
        profile_resp = await core.bot_profile.GetActiveBotProfile(
            bot_profile_pb2.GetActiveBotProfileRequest(tenant_id=tenant_id, channel=channel),
            metadata=metadata,
        )
    except Exception as exc:  # noqa: BLE001 - surfaced as a domain error below
        raise ToolAssemblyError(f"no active bot profile for tenant={tenant_id} channel={channel}: {exc}") from exc

    profile = profile_resp.bot_profile
    allowed_roles = {ROLE_ENUM_TO_NAME.get(r, "") for r in profile.roles_allowed}
    if role not in allowed_roles:
        raise ToolAssemblyError(f"role {role!r} is not permitted to use bot profile {profile.name!r}")

    connectors_resp = await core.connector.ListConnectors(connector_pb2.ListConnectorsRequest(tenant_id=tenant_id))
    wanted_ids = set(profile.connector_ids)
    connectors = [c for c in connectors_resp.connectors if c._id in wanted_ids]

    # Deliberately not gated on connector.status: that field reflects
    # whether core's own cached manifest (RefreshManifest) last
    # succeeded, which is a separate concern from whether orchestrator
    # can reach the connector right now — and in fact can't always agree
    # with it, since core and orchestrator may sit in different network
    # contexts (e.g. core containerized, orchestrator not) and reach a
    # connector by different paths. The live tools/list call below is the
    # authoritative liveness/validity check for this turn, matching
    # docs/architecture/ARCHITECTURE.md §3's diagram, which has Tool
    # Assembly call MCP tools/list directly rather than trusting a cache.
    assembled: list[AssembledTool] = []
    for connector in connectors:
        try:
            mcp_tools = await list_tools(connector.endpoint)
        except Exception:  # noqa: BLE001
            # A down/misbehaving connector is dropped for this turn, not a
            # hard failure of the whole request (docs/architecture/
            # ARCHITECTURE.md §3's failure-handling rule).
            continue
        for tool in mcp_tools:
            assembled.append(
                AssembledTool(
                    connector_id=connector._id,
                    connector_name=connector.name,
                    endpoint=connector.endpoint,
                    tool=tool,
                )
            )
    return assembled
