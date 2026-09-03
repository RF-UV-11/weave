"""One-time registration script: bootstraps a fresh "Acme Electronics"
tenant against a real running Weave core and attaches this directory's
api.py (assumed already running — see initialize.sh) as tools, split
across an external (customer-facing) and internal (staff-facing) bot
profile. This is the live proof that visibility/category
(docs/architecture/ARCHITECTURE.md §3, PLAN.md Phase 3.7) actually gate
what each profile can do, not just what the proto fields say.

Usage:
    core's server must be running (CORE_ADDR, default localhost:9090),
    and this directory's api.py must be running (DEMO_API_URL, default
    http://localhost:9100) before running this script.

    ./.venv/Scripts/python.exe setup_weave.py

Idempotency: none — re-running creates a second tenant with a fresh
random suffix each time (RegisterHttpTool/CreateBotProfile don't dedupe
across runs). This is a demo bootstrap script, not a migration; delete
the tenant via core if you need to start over.
"""

import asyncio
import os
import secrets

from weave_shared_clients import CoreClient

from core.data_access.v1 import auth_pb2, tenant_pb2

import weave

CORE_ADDR = os.environ.get("CORE_ADDR", "localhost:9090")
DEMO_API_URL = os.environ.get("DEMO_API_URL", "http://localhost:9100")
OWNER_EMAIL = os.environ.get("DEMO_OWNER_EMAIL", "owner@acme-electronics.test")
OWNER_PASSWORD = os.environ.get("DEMO_OWNER_PASSWORD", "hunter2hunter2")


async def _bootstrap_tenant() -> str:
    """Creates a fresh tenant + owner user via a direct core connection
    (CreateTenant/Register are unauthenticated bootstrap RPCs — there's
    no token to present before a tenant/user exists at all). Returns the
    new tenant_id; the rest of this script uses weave.connect() like any
    real integrator would, not this bootstrap client."""
    core = CoreClient(CORE_ADDR)
    try:
        tenant_resp = await core.tenant.CreateTenant(
            tenant_pb2.CreateTenantRequest(
                display_name=f"Acme Electronics ({secrets.token_hex(3)})",
                tenant_type="business",
            )
        )
        tenant_id = tenant_resp.tenant._id
        await core.auth.Register(
            auth_pb2.RegisterRequest(
                tenant_id=tenant_id, email=OWNER_EMAIL, password=OWNER_PASSWORD, role=1  # owner
            )
        )
        return tenant_id
    finally:
        await core.close()


async def main() -> None:
    tenant_id = await _bootstrap_tenant()
    print(f"==> Created tenant {tenant_id}")

    client = await weave.connect_async(
        tenant_id=tenant_id, email=OWNER_EMAIL, password=OWNER_PASSWORD, core_addr=CORE_ADDR
    )
    try:
        # --- External tools: safe for a customer-facing bot -----------
        await client.add_tool(
            name="track_order",
            description=(
                "Look up the shipping status and estimated delivery date for a customer's order, "
                "given the order ID (e.g. ORD-1001)."
            ),
            endpoint=f"{DEMO_API_URL}/orders/{{order_id}}/status",
            method="GET",
            params_schema={
                "type": "object",
                "properties": {"order_id": {"type": "string", "description": "The order ID, e.g. ORD-1001."}},
                "required": ["order_id"],
            },
            visibility="external",
            category="general",
        )
        await client.add_tool(
            name="get_product_info",
            description="Look up a product's name, price, and category by its SKU (e.g. SKU-HP-14).",
            endpoint=f"{DEMO_API_URL}/products/{{sku}}",
            method="GET",
            params_schema={
                "type": "object",
                "properties": {"sku": {"type": "string", "description": "The product SKU, e.g. SKU-HP-14."}},
                "required": ["sku"],
            },
            visibility="external",
            category="general",
        )
        await client.add_tool(
            name="check_warranty",
            description="Check whether an order's warranty is active or void, and its expiry date, given the order ID.",
            endpoint=f"{DEMO_API_URL}/warranty/{{order_id}}",
            method="GET",
            params_schema={
                "type": "object",
                "properties": {"order_id": {"type": "string", "description": "The order ID, e.g. ORD-1001."}},
                "required": ["order_id"],
            },
            visibility="external",
            category="general",
        )

        # --- Internal-only tools: staff bots only ----------------------
        await client.add_tool(
            name="get_customer_details",
            description=(
                "Look up a customer's full contact details (name, email, phone, address) by customer ID. "
                "Contains PII — internal/staff use only."
            ),
            endpoint=f"{DEMO_API_URL}/internal/customers/{{customer_id}}",
            method="GET",
            params_schema={
                "type": "object",
                "properties": {"customer_id": {"type": "string", "description": "The customer ID, e.g. cust_amara."}},
                "required": ["customer_id"],
            },
            visibility="internal",
            category="general",
        )
        await client.add_tool(
            name="get_order_internal_details",
            description=(
                "Look up full internal details for an order, including supplier and cost basis — "
                "internal/staff use only, never expose supplier or cost figures to a customer."
            ),
            endpoint=f"{DEMO_API_URL}/internal/orders/{{order_id}}",
            method="GET",
            params_schema={
                "type": "object",
                "properties": {"order_id": {"type": "string", "description": "The order ID, e.g. ORD-1001."}},
                "required": ["order_id"],
            },
            visibility="internal",
            category="general",
        )
        await client.add_tool(
            name="check_inventory",
            description="List current stock levels and reorder thresholds for every product SKU. Internal/staff use only.",
            endpoint=f"{DEMO_API_URL}/internal/inventory",
            method="GET",
            visibility="internal",
            category="general",
        )
        await client.add_tool(
            name="get_sales_report",
            description=(
                "Get an aggregate sales report: revenue, order counts, gross margin, and the top-selling "
                "product for a given period. Internal/staff use only."
            ),
            endpoint=f"{DEMO_API_URL}/internal/analytics/sales",
            method="GET",
            params_schema={
                "type": "object",
                "properties": {"period": {"type": "string", "description": "Reporting period, e.g. 'this_month'."}},
            },
            visibility="internal",
            category="analytics",
        )
        await client.add_tool(
            name="get_customer_activity_report",
            description=(
                "Get an aggregate customer-activity report: how many active, new, and returning customers "
                "there were in a given period. Internal/staff use only."
            ),
            endpoint=f"{DEMO_API_URL}/internal/analytics/customers",
            method="GET",
            params_schema={
                "type": "object",
                "properties": {"period": {"type": "string", "description": "Reporting period, e.g. 'this_month'."}},
            },
            visibility="internal",
            category="analytics",
        )
        print("==> Registered 8 tools (3 external, 5 internal)")

        external_profile = await client.create_bot_profile(
            name="external",
            persona="personas/external.md",
            channels=["web-widget"],
            roles_allowed=["customer"],
            visibility="external",
            guardrails=[
                "Never disclose supplier names.",
                "Never disclose cost basis, margins, or internal pricing figures.",
                "Never disclose another customer's contact details.",
            ],
            web_search_enabled=True,
        )
        internal_profile = await client.create_bot_profile(
            name="internal",
            persona="personas/internal.md",
            channels=["slack"],
            roles_allowed=["staff", "admin", "owner"],
            visibility="internal",
            web_search_enabled=False,
        )
        print(f"==> Created bot profiles: external={external_profile.id} internal={internal_profile.id}")
        print()
        print(f"tenant_id={tenant_id}")
        print(f"owner_email={OWNER_EMAIL}")
        print(f"owner_password={OWNER_PASSWORD}")
    finally:
        await client.close()


if __name__ == "__main__":
    asyncio.run(main())
