"""Not part of the gateway itself — a throwaway stand-in for "a
business's real public-facing API," used only for manual end-to-end
verification of the weave SDK -> core -> mcp-gateway -> orchestrator
path. Run with: python tests/fake_business_api.py [port]
"""

import json
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer

ORDERS = {
    "123": {"status": "shipped", "eta": "2026-08-20"},
    "456": {"status": "processing", "eta": "2026-08-25"},
}


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        parts = [p for p in self.path.split("/") if p]
        if len(parts) == 3 and parts[0] == "orders" and parts[2] == "status":
            order_id = parts[1]
            order = ORDERS.get(order_id)
            payload = order if order else {"error": f"no such order {order_id}"}
            status = 200 if order else 404
        else:
            payload = {"error": "not found"}
            status = 404

        body = json.dumps(payload).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, fmt, *args):
        pass


if __name__ == "__main__":
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 9001
    HTTPServer(("0.0.0.0", port), Handler).serve_forever()
