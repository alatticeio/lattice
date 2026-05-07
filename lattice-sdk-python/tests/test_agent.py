import pytest
import httpx
import respx
from lattice_sdk import LatticeAgent


@pytest.mark.anyio
async def test_agent_enrolls_on_enter():
    with respx.mock:
        respx.post("https://lattice.test/api/v1/agent-enroll").mock(
            return_value=httpx.Response(
                200,
                json={
                    "data": {
                        "peerName": "agent-test-001",
                        "enrollmentToken": "lt-abc123",
                        "expiresAt": "2026-05-06T12:00:00Z",
                    }
                },
            )
        )
        respx.delete(
            url__regex=r"https://lattice\.test/api/v1/agent-enroll/agent-test-001"
        ).mock(return_value=httpx.Response(200, json={"data": None}))

        async with LatticeAgent(
            server="https://lattice.test",
            token="ws-token",
            workspace_id="ws-123",
            agent_name="test-001",
            agent_type="code-executor",
            ttl_seconds=3600,
        ) as agent:
            assert agent.peer_name == "agent-test-001"
            assert agent.enrollment_token == "lt-abc123"


@pytest.mark.anyio
async def test_agent_revokes_on_exit():
    revoke_called = []

    with respx.mock:
        respx.post("https://lattice.test/api/v1/agent-enroll").mock(
            return_value=httpx.Response(
                200,
                json={
                    "data": {
                        "peerName": "agent-cleanup-001",
                        "enrollmentToken": "lt-xyz",
                        "expiresAt": "2026-05-06T12:00:00Z",
                    }
                },
            )
        )

        def capture_revoke(request, route):
            revoke_called.append(request.url.path)
            return httpx.Response(200, json={"data": None})

        respx.delete(
            url__regex=r"https://lattice\.test/api/v1/agent-enroll/agent-cleanup-001"
        ).mock(side_effect=capture_revoke)

        async with LatticeAgent(
            server="https://lattice.test",
            token="ws-token",
            workspace_id="ws-123",
            agent_name="cleanup-001",
            agent_type="code-executor",
        ):
            pass

    assert any("agent-cleanup-001" in path for path in revoke_called), (
        "DELETE should have been called on exit"
    )
