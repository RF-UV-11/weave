"""Acme Electronics — a demo vendor's real public-facing API
(PLAN.md's Phase 3.8), standing in for "a business that already runs its
own systems and wants Weave to reason over them without building an MCP
server." Every route here is a genuine HTTP endpoint a real business
could run; setup_weave.py is what turns a subset of them into Weave
tools via the weave SDK's add_tool(), with visibility/category exactly
as a real integrator would set them (docs/architecture/ARCHITECTURE.md
§3).

Deliberate design choice for the visibility split: sensitive fields
(supplier, cost_basis_usd, customer PII) live only on internal-only
routes, never as extra fields on an external route's response. This is
the "mark specific fields sensitive so they're stripped before
returning" approach docs/architecture/SECURITY.md §7 flags as a real
follow-up to whole-blob guardrail redaction — done here at the API
surface itself, one level earlier than a guardrail would need to catch it.
"""

from fastapi import FastAPI, HTTPException

from data import CUSTOMERS, INVENTORY, ORDERS, PRODUCTS, WARRANTIES, customer_report, sales_report

app = FastAPI(title="Acme Electronics API", description="Demo vendor backing Weave's end-to-end simulation")

# --------------------------------------------------------------------
# External / customer-facing routes — safe to register as visibility=
# "external" tools. No supplier, cost, or PII fields anywhere below.
# --------------------------------------------------------------------


@app.get("/orders/{order_id}/status")
def get_order_status(order_id: str):
    order = ORDERS.get(order_id)
    if not order:
        raise HTTPException(status_code=404, detail=f"no such order {order_id}")
    return {"order_id": order["order_id"], "status": order["status"], "eta": order["eta"]}


@app.get("/products/{sku}")
def get_product(sku: str):
    product = PRODUCTS.get(sku)
    if not product:
        raise HTTPException(status_code=404, detail=f"no such product {sku}")
    return product


@app.get("/warranty/{order_id}")
def get_warranty(order_id: str):
    warranty = WARRANTIES.get(order_id)
    if not warranty:
        raise HTTPException(status_code=404, detail=f"no warranty record for order {order_id}")
    return warranty


# --------------------------------------------------------------------
# Internal-only routes — visibility="internal" tools. Staff bot profiles
# can use these; a customer-facing external profile never sees them, at
# the tool-assembly stage (orchestrator/tools/assembly.py), not via a
# guardrail catching it after the fact.
# --------------------------------------------------------------------


@app.get("/internal/customers/{customer_id}")
def get_customer(customer_id: str):
    customer = CUSTOMERS.get(customer_id)
    if not customer:
        raise HTTPException(status_code=404, detail=f"no such customer {customer_id}")
    return customer


@app.get("/internal/orders/{order_id}")
def get_order_internal(order_id: str):
    """Same order, full detail — including supplier and cost basis,
    fields the external /orders/{id}/status route never exposes."""
    order = ORDERS.get(order_id)
    if not order:
        raise HTTPException(status_code=404, detail=f"no such order {order_id}")
    return order


@app.get("/internal/inventory")
def list_inventory():
    return {"items": list(INVENTORY.values())}


@app.get("/internal/inventory/{sku}")
def get_inventory_item(sku: str):
    item = INVENTORY.get(sku)
    if not item:
        raise HTTPException(status_code=404, detail=f"no inventory record for {sku}")
    return item


@app.get("/internal/analytics/sales")
def get_sales_analytics(period: str = "current"):
    return sales_report(period)


@app.get("/internal/analytics/customers")
def get_customer_analytics(period: str = "current"):
    return customer_report(period)


if __name__ == "__main__":
    import os

    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=int(os.environ.get("DEMO_PORT", "9100")))
