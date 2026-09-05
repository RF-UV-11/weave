import pytest

from weave.openapi import OpenApiRegistrationError, tools_from_openapi

SPEC = {
    "servers": [{"url": "https://api.acme.example"}],
    "paths": {
        "/orders/{order_id}/status": {
            "get": {
                "operationId": "track_order",
                "summary": "Look up shipping status for an order.",
                "parameters": [
                    {"name": "order_id", "in": "path", "required": True, "schema": {"type": "string"}},
                ],
            }
        },
        "/products/{sku}": {
            "get": {
                "operationId": "get_product_info",
                "description": "Look up a product's name and price by SKU.",
                "parameters": [
                    {"name": "sku", "in": "path", "required": True, "schema": {"type": "string"}},
                ],
                "x-weave-visibility": "external",
            }
        },
        "/internal/customers/{customer_id}": {
            "get": {
                "operationId": "get_customer_details",
                "description": "Full customer PII — internal only.",
                "parameters": [{"name": "customer_id", "in": "path", "required": True, "schema": {"type": "string"}}],
            }
        },
        "/internal/analytics/sales": {
            "get": {
                "operationId": "get_sales_report",
                "description": "Aggregate sales report.",
                "parameters": [{"name": "period", "in": "query", "required": False, "schema": {"type": "string"}}],
                "x-weave-category": "analytics",
            }
        },
        "/internal/debug/dump": {
            "delete": {
                "operationId": "internal_debug_dump",
                "description": "Wipes the debug cache. Never expose this.",
            }
        },
    },
}


def test_include_registers_only_the_named_subset():
    planned = tools_from_openapi(SPEC, include={"track_order", "get_product_info"})
    assert {p.name for p in planned} == {"track_order", "get_product_info"}


def test_exclude_registers_everything_except_the_named_subset():
    planned = tools_from_openapi(SPEC, exclude={"internal_debug_dump"})
    assert {p.name for p in planned} == {
        "track_order",
        "get_product_info",
        "get_customer_details",
        "get_sales_report",
    }


def test_passing_neither_registers_every_operation():
    planned = tools_from_openapi(SPEC)
    assert len(planned) == 5


def test_include_and_exclude_together_is_rejected():
    with pytest.raises(OpenApiRegistrationError, match="at most one"):
        tools_from_openapi(SPEC, include={"track_order"}, exclude={"internal_debug_dump"})


def test_base_url_defaults_from_spec_servers():
    planned = tools_from_openapi(SPEC, include={"track_order"})
    assert planned[0].endpoint == "https://api.acme.example/orders/{order_id}/status"


def test_base_url_override_takes_precedence():
    planned = tools_from_openapi(SPEC, include={"track_order"}, base_url="http://localhost:9101")
    assert planned[0].endpoint == "http://localhost:9101/orders/{order_id}/status"


def test_path_param_becomes_required_params_schema_property():
    planned = tools_from_openapi(SPEC, include={"track_order"})
    schema = planned[0].params_schema
    assert schema["properties"]["order_id"]["type"] == "string"
    assert schema["required"] == ["order_id"]


def test_optional_query_param_not_marked_required():
    planned = tools_from_openapi(SPEC, include={"get_sales_report"})
    schema = planned[0].params_schema
    assert "period" in schema["properties"]
    assert "required" not in schema


def test_method_uppercased():
    planned = tools_from_openapi(SPEC, include={"internal_debug_dump"})
    assert planned[0].method == "DELETE"


def test_visibility_defaults_to_internal_unless_overridden():
    planned = tools_from_openapi(SPEC, include={"track_order", "get_product_info"})
    by_name = {p.name: p for p in planned}
    assert by_name["track_order"].visibility == "internal"
    assert by_name["get_product_info"].visibility == "external"


def test_category_override_via_extension_key():
    planned = tools_from_openapi(SPEC, include={"track_order", "get_sales_report"})
    by_name = {p.name: p for p in planned}
    assert by_name["track_order"].category == "general"
    assert by_name["get_sales_report"].category == "analytics"


def test_default_visibility_and_category_can_be_overridden_spec_wide():
    planned = tools_from_openapi(
        SPEC, include={"track_order"}, default_visibility="external", default_category="analytics"
    )
    assert planned[0].visibility == "external"
    assert planned[0].category == "analytics"


def test_operation_missing_description_and_summary_is_rejected():
    spec = {
        "servers": [{"url": "https://api.acme.example"}],
        "paths": {"/ping": {"get": {"operationId": "ping"}}},
    }
    with pytest.raises(OpenApiRegistrationError, match="no description or summary"):
        tools_from_openapi(spec)


def test_operation_missing_description_is_fine_if_not_selected():
    spec = {
        "servers": [{"url": "https://api.acme.example"}],
        "paths": {
            "/ping": {"get": {"operationId": "ping"}},
            "/orders/{order_id}/status": SPEC["paths"]["/orders/{order_id}/status"],
        },
    }
    # "ping" has no description but isn't in `include`, so it's never
    # validated — only operations actually being registered are checked.
    planned = tools_from_openapi(spec, include={"track_order"})
    assert [p.name for p in planned] == ["track_order"]


def test_auth_mode_defaults_to_none():
    planned = tools_from_openapi(SPEC, include={"track_order"})
    assert planned[0].auth_mode == "none"


def test_auth_mode_override_via_extension_key():
    spec = {
        "servers": [{"url": "https://finance.example"}],
        "paths": {
            "/me/transactions": {
                "get": {
                    "operationId": "get_my_transactions",
                    "description": "The caller's own transactions.",
                    "x-weave-auth-mode": "user_token",
                }
            }
        },
    }
    planned = tools_from_openapi(spec)
    assert planned[0].auth_mode == "user_token"


def test_default_auth_mode_applies_spec_wide():
    spec = {
        "servers": [{"url": "https://finance.example"}],
        "paths": {"/me/balance": {"get": {"operationId": "get_my_balance", "description": "The caller's own balance."}}},
    }
    planned = tools_from_openapi(spec, default_auth_mode="user_token")
    assert planned[0].auth_mode == "user_token"


def test_operation_id_falls_back_to_method_and_path_when_unset():
    spec = {
        "servers": [{"url": "https://api.acme.example"}],
        "paths": {"/widgets/{id}": {"get": {"description": "Get a widget."}}},
    }
    planned = tools_from_openapi(spec)
    assert planned[0].name == "get_widgets_id"


def test_non_method_keys_in_a_path_item_are_ignored():
    spec = {
        "servers": [{"url": "https://api.acme.example"}],
        "paths": {
            "/widgets/{id}": {
                "parameters": [{"name": "id", "in": "path", "required": True, "schema": {"type": "string"}}],
                "get": {"operationId": "get_widget", "description": "Get a widget."},
            }
        },
    }
    planned = tools_from_openapi(spec)
    assert [p.name for p in planned] == ["get_widget"]
