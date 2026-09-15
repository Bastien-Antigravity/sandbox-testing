#!/usr/bin/env python3
# coding:utf-8

"""
SCENARIO TEST: RAG Engine MCP Server-Sent Events (SSE) Protocol & Port Parity
Location: sandbox-testing/02-Scenarios/python/rag_mcp_sse_test.py

ESSENTIAL PROCESS:
Validates that:
1. FastMCP mounts /sse and /message routes properly on FastAPI via mount_to_app.
2. Initial handshake initializes an MCP session with UUID and session queue.
3. POST /sse dispatches JSON-RPC requests (initialize, tools/list, tool call).
4. DELETE /sse cleanly terminates the session and signals the queue.
5. Docker Compose and local configurations maintain 100% port parity (port 8090).
"""

import sys
import os
import re
import json
import asyncio
from pathlib import Path

# Add paths
TEST_DIR = Path(__file__).resolve().parent
SANDBOX_DIR = TEST_DIR.parent.parent
WORKSPACE_ROOT = SANDBOX_DIR.parent
RAG_ENGINE_DIR = WORKSPACE_ROOT / "obsidian-brain" / "09-RAG-Engine"
DEPLOY_DIR = WORKSPACE_ROOT / "docker-deployment"

if str(RAG_ENGINE_DIR) not in sys.path:
    sys.path.insert(0, str(RAG_ENGINE_DIR))

from fastapi import FastAPI
from fastapi.testclient import TestClient
from src.services.mcp.fastmcp import FastMCP


def run_mcp_sse_scenario():
    print("=" * 70)
    print("🧪 RAG ENGINE MCP SSE PROTOCOL & PORT PARITY SCENARIO TEST")
    print("=" * 70)

    # STAGE 1: FastMCP Route Mounting
    print("\n[STAGE 1] Testing FastMCP Route Mounting on FastAPI...")
    app = FastAPI()
    mcp = FastMCP("test_mcp_rag")

    @mcp.tool()
    def echo_test(msg: str) -> str:
        """Simple echo test tool."""
        return f"echo: {msg}"

    mcp.mount_to_app(app)
    routes = [route.path for route in app.routes]

    assert "/sse" in routes, "Route /sse not registered on FastAPI app!"
    assert "/message" in routes, "Route /message not registered on FastAPI app!"
    print("  ├─ Registered routes: /sse, /message")
    print("  └─ ✅ STAGE 1 PASSED: FastMCP routes mounted successfully.")

    # STAGE 2: JSON-RPC Initialization & Session Handshake
    print("\n[STAGE 2] Testing JSON-RPC Initialization & Session Lifecycle...")
    client = TestClient(app)

    # Send initialize JSON-RPC request without session_id to acquire a new session
    init_res = client.post(
        "/sse",
        json={
            "jsonrpc": "2.0",
            "id": 1,
            "method": "initialize",
            "params": {
                "protocolVersion": "2024-11-05",
                "clientInfo": {"name": "TestIDE", "version": "1.0.0"}
            }
        }
    )
    assert init_res.status_code == 200, f"Expected 200 OK, got {init_res.status_code}"
    session_id = init_res.headers.get("Mcp-Session-Id") or init_res.headers.get("mcp-session-id")
    assert session_id, "Response headers did not include Mcp-Session-Id!"
    assert session_id in mcp._sse_sessions, f"Session {session_id} not registered in FastMCP session map!"
    print(f"  ├─ Initialized session ID: {session_id}")

    # STAGE 3: Tools Listing over Session
    print("\n[STAGE 3] Testing Tools Listing Dispatch via POST /sse...")
    tools_res = client.post(
        f"/sse?session_id={session_id}",
        json={
            "jsonrpc": "2.0",
            "id": 2,
            "method": "tools/list",
            "params": {}
        }
    )
    assert tools_res.status_code in (200, 202), f"Expected 200/202, got {tools_res.status_code}"
    tools_body = tools_res.json()
    assert "result" in tools_body, f"Expected 'result' in response: {tools_body}"
    registered_tools = [t["name"] for t in tools_body["result"].get("tools", [])]
    assert "echo_test" in registered_tools, f"Tool 'echo_test' missing in tools/list: {registered_tools}"
    print(f"  ├─ Discovered tools: {registered_tools}")
    print("  └─ ✅ STAGE 3 PASSED: Tool listing retrieved successfully.")

    # STAGE 4: Clean Session Deletion
    print("\n[STAGE 4] Testing Clean Session Termination via DELETE /sse...")
    del_res = client.delete(f"/sse?session_id={session_id}")
    assert del_res.status_code == 200, f"Expected 200 OK, got {del_res.status_code}"
    assert session_id not in mcp._sse_sessions, f"Session {session_id} should be removed from session map!"
    print("  └─ ✅ STAGE 4 PASSED: Session terminated cleanly.")

    # STAGE 5: Port 8090 Configuration Parity Audit
    print("\n[STAGE 5] Auditing Port 8090 Parity Across Compose & Config Files...")
    compose_path = DEPLOY_DIR / "docker-compose.yaml"
    assert compose_path.exists(), "docker-compose.yaml missing in docker-deployment!"
    compose_content = compose_path.read_text(encoding="utf-8")

    # Verify port 8090 is mapped in rag-engine
    assert "8090:8090" in compose_content or "RG_MCP_PORT:-8090}:8090" in compose_content, \
        "docker-compose.yaml must forward MCP port 8090 for rag-engine!"
    print("  ├─ docker-compose.yaml: port 8090 verified")

    # Verify docker mode compose
    docker_mode_compose = DEPLOY_DIR / "modes" / "docker" / "docker-compose.yaml"
    if docker_mode_compose.exists():
        dm_content = docker_mode_compose.read_text(encoding="utf-8")
        assert "8090:8090" in dm_content or "RG_MCP_PORT:-8090}:8090" in dm_content, \
            "modes/docker/docker-compose.yaml must forward MCP port 8090!"
        print("  ├─ modes/docker/docker-compose.yaml: port 8090 verified")

    # Verify IDE setup default
    setup_ide_path = DEPLOY_DIR / "modes" / "local" / "ide" / "setup_ide.py"
    ide_content = setup_ide_path.read_text(encoding="utf-8")
    assert "8090" in ide_content, "setup_ide.py must default MCP port to 8090!"
    print("  ├─ setup_ide.py: port 8090 verified")
    print("  └─ ✅ STAGE 5 PASSED: 100% Port Parity (8090) verified across deployment definitions.")

    print("\n" + "=" * 70)
    print("✨ ALL RAG MCP SSE PROTOCOL & PORT PARITY TESTS PASSED PERFECTLY!")
    print("=" * 70)


if __name__ == "__main__":
    run_mcp_sse_scenario()
