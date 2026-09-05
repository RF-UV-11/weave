from llm import ollama_client, openai_compat_client
from llm.router import get_provider


def test_empty_string_resolves_to_ollama():
    assert get_provider("") is ollama_client


def test_ollama_resolves_to_ollama_client():
    assert get_provider("ollama") is ollama_client


def test_openai_resolves_to_openai_compat_client():
    assert get_provider("openai") is openai_compat_client


def test_unrecognized_provider_falls_back_to_ollama():
    # Reachable only via a profile stored before this field existed, or a
    # future core bug — core itself rejects anything else at
    # CreateBotProfile time (rpc_services/bot_profile/routehandler.go).
    assert get_provider("claude") is ollama_client


def test_every_provider_module_exposes_the_same_chat_shape():
    # router.py's whole point is that callers never branch on provider —
    # this is the contract that makes that safe.
    for provider in (ollama_client, openai_compat_client):
        assert callable(provider.chat)
        assert callable(provider.chat_stream)
