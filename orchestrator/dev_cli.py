#!/usr/bin/env python3
"""Minimal dev harness for Phase 3's definition of done — not the real
`web` app, just enough to watch a chat turn happen end-to-end: logs into
core, opens a ChatStream to orchestrator, prints the streamed answer.

Usage:
  python dev_cli.py --tenant-id tnt_... --email owner@acme.test --password ... \
      --channel web-widget "Book me an appointment for 2026-08-20 at 3pm, name Ada"
"""

import argparse
import asyncio
import sys
from pathlib import Path

_ORCH_DIR = Path(__file__).parent
sys.path.insert(0, str(_ORCH_DIR))
sys.path.insert(0, str(_ORCH_DIR / "gen"))

import grpc  # noqa: E402

from gen.orchestrator.v1 import chat_pb2, chat_pb2_grpc  # noqa: E402
from weave_shared_clients import CoreClient  # noqa: E402
from core.data_access.v1 import auth_pb2  # noqa: E402


async def login(core: CoreClient, tenant_id: str, email: str, password: str) -> str:
    resp = await core.auth.Login(auth_pb2.LoginRequest(tenant_id=tenant_id, email=email, password=password))
    return resp.access_token


async def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("message")
    parser.add_argument("--tenant-id", required=True)
    parser.add_argument("--email", required=True)
    parser.add_argument("--password", required=True)
    parser.add_argument("--channel", default="web-widget")
    parser.add_argument("--core-addr", default="localhost:9090")
    parser.add_argument("--orchestrator-addr", default="localhost:9091")
    args = parser.parse_args()

    core = CoreClient(args.core_addr)
    token = await login(core, args.tenant_id, args.email, args.password)
    await core.close()

    channel = grpc.aio.insecure_channel(args.orchestrator_addr)
    stub = chat_pb2_grpc.ChatServiceStub(channel)
    request = chat_pb2.ChatStreamRequest(channel=args.channel, message=args.message)
    metadata = (("authorization", f"Bearer {token}"),)

    print(f"> {args.message}\n")
    tool_used = ""
    async for resp in stub.ChatStream(request, metadata=metadata):
        if resp.tool_used and not tool_used:
            tool_used = resp.tool_used
            print(f"[tool used: {resp.tool_used} via connector {resp.connector_used}]\n")
        if resp.token:
            print(resp.token, end="", flush=True)
    print()

    await channel.close()


if __name__ == "__main__":
    asyncio.run(main())
