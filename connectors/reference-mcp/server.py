"""Reference MCP server: a fake booking system.

Not a real connector template — just enough of one to develop and verify
the orchestrator's MCP client against (docs/architecture/ARCHITECTURE.md
§3's dynamic tool assembly needs *something* real to discover and call).
Tool descriptions are mandatory, not decoration (see the docstrings below
and PLAN.md's tool-description requirement) — core rejects caching a
manifest for any tool that lacks one.
"""

import os

from mcp.server import MCPServer

server = MCPServer(name="reference-booking-mcp", version="0.1.0")

# In-memory only — restarts empty. This is a dev fixture, not a real
# booking system.
_bookings: dict[str, dict[str, str]] = {}


@server.tool()
def book_appointment(date: str, time: str, customer_name: str) -> str:
    """Book an appointment slot for a customer on a given date and time.

    Args:
        date: Appointment date, e.g. "2026-08-20".
        time: Appointment time, e.g. "14:30".
        customer_name: Name of the customer the appointment is for.
    """
    booking_id = f"bkg_{len(_bookings) + 1}"
    _bookings[booking_id] = {"date": date, "time": time, "customer_name": customer_name}
    return f"Booked {booking_id}: {customer_name} on {date} at {time}."


@server.tool()
def cancel_appointment(booking_id: str) -> str:
    """Cancel an existing appointment by its booking ID.

    Args:
        booking_id: The booking ID returned by book_appointment, e.g. "bkg_1".
    """
    if booking_id not in _bookings:
        return f"No such booking: {booking_id}."
    del _bookings[booking_id]
    return f"Cancelled {booking_id}."


@server.tool()
def list_appointments() -> str:
    """List all currently booked appointments."""
    if not _bookings:
        return "No appointments booked."
    return "\n".join(
        f"{bid}: {b['customer_name']} on {b['date']} at {b['time']}" for bid, b in _bookings.items()
    )


if __name__ == "__main__":
    server.run(transport="streamable-http", host="0.0.0.0", port=int(os.environ.get("MCP_PORT", "8765")))
