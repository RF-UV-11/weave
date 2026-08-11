"""Trivial JSON-RPC-over-HTTP stub MCP server for exercising core's
RefreshManifest RPC in dev (Phase 1). Not a real MCP transport (no
initialize handshake, no SSE) — just enough tools/list surface for
core/mcpclient to cache a manifest against. Real MCP client work happens
in orchestrator (Phase 3).

Usage: python server.py [port]  (default port 8765)
"""

import json
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer

TOOLS = [
    {
        "name": "book_appointment",
        "description": "Book an appointment slot",
        "inputSchema": {
            "type": "object",
            "properties": {"date": {"type": "string"}, "time": {"type": "string"}},
            "required": ["date", "time"],
        },
    }
]


class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        body = json.loads(self.rfile.read(length) or b"{}")
        method = body.get("method")

        if method == "tools/list":
            result = {"tools": TOOLS}
            response = {"jsonrpc": "2.0", "id": body.get("id"), "result": result}
        else:
            response = {
                "jsonrpc": "2.0",
                "id": body.get("id"),
                "error": {"code": -32601, "message": f"method not found: {method}"},
            }

        payload = json.dumps(response).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def log_message(self, fmt, *args):
        pass


if __name__ == "__main__":
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 8765
    HTTPServer(("0.0.0.0", port), Handler).serve_forever()
