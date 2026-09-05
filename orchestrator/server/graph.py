"""The planner -> specialist-agent -> tool-call state machine from
docs/architecture/ARCHITECTURE.md §2's sequence diagram, as a small
LangGraph graph.

Scope, deliberately minimal for Phase 3's proof (PLAN.md): at most one
tool call per turn. This graph's only job is *deciding* whether a tool is
needed and, if so, executing it — it never generates the user-visible
answer itself. That's `chat_service.py`'s job, via a single real
streaming call on whatever message list this graph settles on, so the
model is never asked to generate the same answer content twice (once to
decide, once to stream).

Guardrail screening (docs/architecture/SECURITY.md §6) happens here too,
at the point a tool's result would enter the model's context — not only
on the final answer in chat_service.py. A tool result that violates a
guardrail is replaced with a redacted note before it ever reaches the
model, rather than trusting the final-answer check alone to catch it.
"""

from typing import Any, TypedDict

from langgraph.graph import END, StateGraph

from llm.base import ToolCall
from llm.router import get_provider
from mcp_client import call_tool
from server.guardrails import screen
from server.web_search import is_web_search, run_web_search
from tools.assembly import AssembledTool


class ChatState(TypedDict):
    messages: list[dict[str, str]]
    available_tools: list[AssembledTool]
    guardrails: list[str]
    llm_provider: str
    llm_model: str
    user_assertion: str
    tool_used: str
    connector_used: str
    pending_call: ToolCall | None


def _tool_choices(tools: list[AssembledTool]) -> list[tuple[str, str, dict[str, Any]]]:
    return [(t.tool.name, t.tool.description, t.tool.input_schema) for t in tools]


async def _agent_node(state: ChatState) -> dict[str, Any]:
    if not state["available_tools"]:
        return {"pending_call": None}

    provider = get_provider(state["llm_provider"])
    result = await provider.chat(
        state["messages"], tools=_tool_choices(state["available_tools"]), model=state["llm_model"] or None
    )
    if result.tool_calls:
        return {"pending_call": result.tool_calls[0]}

    # No tool needed — result.content is discarded on purpose. This call
    # only ever exists to make the tool-or-not decision; the actual
    # user-visible answer is generated exactly once, by a real streaming
    # call in chat_service.py, on state["messages"] as-is.
    return {"pending_call": None}


async def _run_tool(match: AssembledTool, call: ToolCall, user_assertion: str) -> str:
    if is_web_search(match.tool.name):
        return await run_web_search(call.arguments.get("query", ""))
    return await call_tool(match.endpoint, call.name, call.arguments, user_assertion=user_assertion or None)


async def _tool_node(state: ChatState) -> dict[str, Any]:
    call = state["pending_call"]
    assert call is not None
    match = next((t for t in state["available_tools"] if t.tool.name == call.name), None)

    if match is None:
        # The model hallucinated a tool name that isn't in the assembled
        # set — tell it so plainly rather than crashing the turn.
        tool_message = {"role": "tool", "content": f"Tool {call.name!r} is not available."}
        connector_used = ""
    else:
        result_text = await _run_tool(match, call, state["user_assertion"])

        if state["guardrails"]:
            verdict = await screen(result_text, state["guardrails"])
            if not verdict.ok:
                # Redacted before it ever reaches the model's context —
                # the final-answer check in chat_service.py is a second
                # line of defense, not the only one.
                result_text = "[content withheld: this tool result could not be verified against policy]"

        # The tool's description travels with its result — the model
        # (and, via ChatStreamResponse.tool_used, the caller) never sees
        # a raw result without knowing what the tool claims to do
        # (PLAN.md's tool-description requirement, ARCHITECTURE.md §3).
        tool_message = {
            "role": "tool",
            "content": f"{match.tool.description}\n\nResult: {result_text}",
        }
        connector_used = match.connector_name

    return {
        "messages": [*state["messages"], tool_message],
        "tool_used": call.name,
        "connector_used": connector_used,
    }


def _route_after_agent(state: ChatState) -> str:
    return "tool" if state.get("pending_call") is not None else END


def build_graph():
    graph = StateGraph(ChatState)
    graph.add_node("agent", _agent_node)
    graph.add_node("tool", _tool_node)
    graph.set_entry_point("agent")
    graph.add_conditional_edges("agent", _route_after_agent, {"tool": "tool", END: END})
    graph.add_edge("tool", END)
    return graph.compile()


_GRAPH = build_graph()


async def run_turn(
    messages: list[dict[str, str]],
    available_tools: list[AssembledTool],
    guardrails: list[str] | None = None,
    *,
    llm_provider: str = "",
    llm_model: str = "",
    user_assertion: str = "",
) -> ChatState:
    """Decides whether a tool is needed and, if so, calls it. Returns the
    final state — state["messages"] is ready for the caller to stream a
    real answer from (see chat_service.py), and state["tool_used"]/
    ["connector_used"] are set if a tool was called.

    llm_provider/llm_model come from the active bot profile
    (tools/assembly.py's AssemblyResult) and select which LLM backend
    makes the tool-or-not decision here (llm/router.py) — the same
    provider/model chat_service.py then uses for the actual answer, so a
    turn never mixes two different backends' opinions about the same
    tool call.

    user_assertion (server/auth.py's mint_user_assertion, minted once per
    turn from the caller's own JWT claims) is forwarded on every tool
    call this turn makes (server/graph.py's _run_tool); only a tool whose
    HttpTool.auth_mode == "user_token" ever acts on it
    (mcp-gateway/gateway/tenant_server.py) — every other tool ignores it."""
    initial: ChatState = {
        "messages": messages,
        "available_tools": available_tools,
        "guardrails": guardrails or [],
        "llm_provider": llm_provider,
        "llm_model": llm_model,
        "user_assertion": user_assertion,
        "tool_used": "",
        "connector_used": "",
        "pending_call": None,
    }
    return await _GRAPH.ainvoke(initial)
