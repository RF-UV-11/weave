"""In-memory canned data for the Acme Electronics demo vendor
(docs — PLAN.md's Phase 3.8). Not a real business system: this is
deliberately a fixed, restart-empty dataset just realistic enough to
demonstrate the external/internal tool-visibility split and give the
analytics endpoints something non-trivial to aggregate over.
"""

from datetime import date

ORDERS: dict[str, dict] = {
    "ORD-1001": {
        "order_id": "ORD-1001",
        "customer_id": "cust_amara",
        "sku": "SKU-HP-14",
        "status": "shipped",
        "eta": "2026-09-08",
        "placed_on": "2026-09-01",
        "total_usd": 1249.00,
        "supplier": "Nova Components Ltd",
        "cost_basis_usd": 812.00,
    },
    "ORD-1002": {
        "order_id": "ORD-1002",
        "customer_id": "cust_devon",
        "sku": "SKU-EB-27",
        "status": "processing",
        "eta": "2026-09-12",
        "placed_on": "2026-09-02",
        "total_usd": 329.00,
        "supplier": "Nova Components Ltd",
        "cost_basis_usd": 201.00,
    },
    "ORD-1003": {
        "order_id": "ORD-1003",
        "customer_id": "cust_amara",
        "sku": "SKU-WH-9",
        "status": "delivered",
        "eta": "2026-08-28",
        "placed_on": "2026-08-20",
        "total_usd": 179.00,
        "supplier": "Silverline Audio",
        "cost_basis_usd": 96.00,
    },
    "ORD-1004": {
        "order_id": "ORD-1004",
        "customer_id": "cust_priya",
        "sku": "SKU-HP-14",
        "status": "delivered",
        "eta": "2026-08-25",
        "placed_on": "2026-08-18",
        "total_usd": 1249.00,
        "supplier": "Nova Components Ltd",
        "cost_basis_usd": 812.00,
    },
    "ORD-1005": {
        "order_id": "ORD-1005",
        "customer_id": "cust_devon",
        "sku": "SKU-MN-32",
        "status": "cancelled",
        "eta": None,
        "placed_on": "2026-08-30",
        "total_usd": 449.00,
        "supplier": "Silverline Audio",
        "cost_basis_usd": 280.00,
    },
}

PRODUCTS: dict[str, dict] = {
    "SKU-HP-14": {"sku": "SKU-HP-14", "name": "Acme Nimbus 14\" Laptop", "price_usd": 1249.00, "category": "laptops"},
    "SKU-EB-27": {"sku": "SKU-EB-27", "name": "Acme Pulse Earbuds", "price_usd": 329.00, "category": "audio"},
    "SKU-WH-9": {"sku": "SKU-WH-9", "name": "Acme Wave Headphones", "price_usd": 179.00, "category": "audio"},
    "SKU-MN-32": {"sku": "SKU-MN-32", "name": "Acme Clarity 32\" Monitor", "price_usd": 449.00, "category": "displays"},
}

INVENTORY: dict[str, dict] = {
    "SKU-HP-14": {"sku": "SKU-HP-14", "on_hand": 14, "reorder_threshold": 10, "warehouse": "WH-EAST"},
    "SKU-EB-27": {"sku": "SKU-EB-27", "on_hand": 3, "reorder_threshold": 15, "warehouse": "WH-EAST"},
    "SKU-WH-9": {"sku": "SKU-WH-9", "on_hand": 42, "reorder_threshold": 20, "warehouse": "WH-WEST"},
    "SKU-MN-32": {"sku": "SKU-MN-32", "on_hand": 0, "reorder_threshold": 8, "warehouse": "WH-WEST"},
}

WARRANTIES: dict[str, dict] = {
    "ORD-1001": {"order_id": "ORD-1001", "warranty_status": "active", "expires_on": "2028-09-01"},
    "ORD-1002": {"order_id": "ORD-1002", "warranty_status": "active", "expires_on": "2028-09-02"},
    "ORD-1003": {"order_id": "ORD-1003", "warranty_status": "active", "expires_on": "2027-08-28"},
    "ORD-1004": {"order_id": "ORD-1004", "warranty_status": "active", "expires_on": "2028-08-25"},
    "ORD-1005": {"order_id": "ORD-1005", "warranty_status": "void", "expires_on": None},
}

CUSTOMERS: dict[str, dict] = {
    "cust_amara": {
        "customer_id": "cust_amara", "name": "Amara Chen", "email": "amara.chen@example.test",
        "phone": "+1-555-0101", "address": "22 Birch St, Springfield", "since": "2025-03-14",
    },
    "cust_devon": {
        "customer_id": "cust_devon", "name": "Devon Ruiz", "email": "devon.ruiz@example.test",
        "phone": "+1-555-0142", "address": "88 Maple Ave, Riverton", "since": "2025-11-02",
    },
    "cust_priya": {
        "customer_id": "cust_priya", "name": "Priya Nair", "email": "priya.nair@example.test",
        "phone": "+1-555-0177", "address": "5 Cedar Ln, Lakeview", "since": "2026-01-20",
    },
}


def sales_report(period: str) -> dict:
    """Aggregates ORDERS into a revenue report. period is accepted but not
    actually used to filter this fixed dataset (there's only one month of
    demo data) — it's part of the tool's schema so the shape matches what
    a real analytics endpoint would take."""
    non_cancelled = [o for o in ORDERS.values() if o["status"] != "cancelled"]
    revenue = sum(o["total_usd"] for o in non_cancelled)
    cost = sum(o["cost_basis_usd"] for o in non_cancelled)
    by_sku: dict[str, int] = {}
    for o in non_cancelled:
        by_sku[o["sku"]] = by_sku.get(o["sku"], 0) + 1
    top_sku = max(by_sku, key=by_sku.get) if by_sku else None
    return {
        "period": period,
        "orders_count": len(non_cancelled),
        "cancelled_count": len(ORDERS) - len(non_cancelled),
        "revenue_usd": round(revenue, 2),
        "gross_margin_usd": round(revenue - cost, 2),
        "top_selling_sku": top_sku,
        "generated_on": date.today().isoformat(),
    }


def customer_report(period: str) -> dict:
    """Aggregates CUSTOMERS/ORDERS into a customer-activity report, same
    period-is-schema-only caveat as sales_report."""
    orders_per_customer: dict[str, int] = {}
    for o in ORDERS.values():
        orders_per_customer[o["customer_id"]] = orders_per_customer.get(o["customer_id"], 0) + 1
    returning = sum(1 for c in orders_per_customer.values() if c > 1)
    return {
        "period": period,
        "active_customers": len(orders_per_customer),
        "returning_customers": returning,
        "new_customers": len(orders_per_customer) - returning,
        "generated_on": date.today().isoformat(),
    }
