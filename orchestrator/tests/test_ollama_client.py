from llm.ollama_client import _to_ollama_tool


def test_to_ollama_tool_shape():
    tool = _to_ollama_tool("book_appointment", "Book a slot", {"type": "object", "properties": {}})
    assert tool == {
        "type": "function",
        "function": {
            "name": "book_appointment",
            "description": "Book a slot",
            "parameters": {"type": "object", "properties": {}},
        },
    }


def test_to_ollama_tool_defaults_empty_schema():
    tool = _to_ollama_tool("noop", "does nothing", {})
    assert tool["function"]["parameters"] == {"type": "object", "properties": {}}
