import httpx

from server.web_search import WEB_SEARCH_TOOL, is_web_search, run_web_search

_SAMPLE_HTML = """
<div class="result">
  <a rel="nofollow" class="result__a" href="https://example.com/weather">
    London Weather &amp; Forecast
  </a>
  <a class="result__snippet">Current conditions and 7-day forecast for London.</a>
</div>
<div class="result">
  <a rel="nofollow" class="result__a" href="https://example.com/weather2">
    BBC Weather - London
  </a>
  <a class="result__snippet">14-day weather forecast for London, UK.</a>
</div>
"""


def transport_for(handler):
    return httpx.MockTransport(handler)


async def test_extracts_titles_and_snippets():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, text=_SAMPLE_HTML)

    result = await run_web_search("london weather", transport=transport_for(handler))
    assert "London Weather & Forecast" in result
    assert "Current conditions and 7-day forecast" in result
    assert "BBC Weather - London" in result


async def test_decodes_html_entities():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, text=_SAMPLE_HTML)

    result = await run_web_search("london weather", transport=transport_for(handler))
    assert "&amp;" not in result
    assert "&" in result  # decoded from &amp;


async def test_no_results_returns_friendly_message():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, text="<html><body>no results here</body></html>")

    result = await run_web_search("a very obscure query", transport=transport_for(handler))
    assert "No web results found" in result
    assert "a very obscure query" in result


async def test_sends_query_as_form_data():
    captured = {}

    def handler(request: httpx.Request) -> httpx.Response:
        captured["body"] = request.content
        return httpx.Response(200, text="")

    await run_web_search("test query", transport=transport_for(handler))
    assert b"test+query" in captured["body"] or b"test%20query" in captured["body"]


async def test_web_search_tool_has_description_and_schema():
    assert WEB_SEARCH_TOOL.tool.description
    assert WEB_SEARCH_TOOL.tool.input_schema["required"] == ["query"]


def test_is_web_search_matches_only_the_web_search_tool_name():
    assert is_web_search("web_search") is True
    assert is_web_search("get_order_status") is False
