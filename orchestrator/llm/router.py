"""Per-bot-profile LLM provider selection — the model-side switch
mirroring mcp-gateway's tool-side one (docs/architecture/ARCHITECTURE.md
§3): a tenant's `BotProfile.llm_provider` picks which backend generates
that profile's turns, defaulting to Ollama when unset. Every provider
module (`ollama_client`, `openai_compat_client`) exposes the identical
`chat()`/`chat_stream()` shape, so callers (`server/graph.py`,
`server/chat_service.py`) never branch on provider themselves — they
resolve a module once via `get_provider()` and call it uniformly,
same as `tools/assembly.py` never cares which MCP server backs a tool.
"""

from types import ModuleType

from . import ollama_client, openai_compat_client

_PROVIDERS: dict[str, ModuleType] = {
    "": ollama_client,
    "ollama": ollama_client,
    "openai": openai_compat_client,
}


def get_provider(name: str) -> ModuleType:
    """Falls back to ollama_client for "" and any unrecognized name.
    `core` already rejects an unrecognized llm_provider at
    CreateBotProfile time (rpc_services/bot_profile/routehandler.go), so
    reaching an unknown value here means a profile stored before this
    field existed, not a live tenant mistake worth failing a chat turn
    over — same fail-soft posture as session/semantic memory
    (server/session_memory.py, server/semantic_memory.py)."""
    return _PROVIDERS.get(name, ollama_client)
