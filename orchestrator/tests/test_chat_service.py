from server.chat_service import DEFAULT_SYSTEM_PROMPT, _build_system_prompt


def test_uses_tenant_persona_when_set():
    prompt = _build_system_prompt("You are Tarang's support bot. Be concise.", [])
    assert prompt == "You are Tarang's support bot. Be concise."


def test_falls_back_to_default_when_persona_unset():
    assert _build_system_prompt("", []) == DEFAULT_SYSTEM_PROMPT


def test_falls_back_to_default_when_persona_is_only_whitespace():
    # A tenant that set persona="" (or never called create_bot_profile
    # with one at all — the field defaults to "") shouldn't silently get
    # an empty/whitespace system prompt; same fallback either way.
    assert _build_system_prompt("   \n", []) == DEFAULT_SYSTEM_PROMPT


def test_appends_relevant_facts_after_persona():
    prompt = _build_system_prompt("You are a support bot.", ["The user's name is Jordan."])
    assert prompt == (
        "You are a support bot.\n\n"
        "Relevant facts you know about this user from past conversations:\n"
        "- The user's name is Jordan."
    )


def test_appends_relevant_facts_after_default_prompt():
    prompt = _build_system_prompt("", ["The user's favorite product is SKU-WH-9."])
    assert prompt.startswith(DEFAULT_SYSTEM_PROMPT + "\n\n")
    assert "- The user's favorite product is SKU-WH-9." in prompt


def test_multiple_facts_each_get_their_own_bullet():
    prompt = _build_system_prompt("Base persona.", ["fact one", "fact two"])
    assert "- fact one\n- fact two" in prompt
