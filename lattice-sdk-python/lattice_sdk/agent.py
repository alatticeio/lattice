"""LatticeAgent — async context manager for AI agent enrollment."""

from __future__ import annotations

import httpx


class LatticeAgent:
    """Enrolls an AI agent into a Lattice workspace on enter, revokes on exit.

    Usage::

        async with LatticeAgent(
            server="https://lattice.company.com",
            token="lt-workspace-token",
            workspace_id="ws-xxx",
            agent_name="code-executor",
            agent_type="code-executor",
            ttl_seconds=3600,
            policy_preset="sandboxed",
        ) as agent:
            # agent.peer_name, agent.enrollment_token are now available
            await my_task()
    """

    def __init__(
        self,
        server: str,
        token: str,
        workspace_id: str,
        agent_name: str,
        agent_type: str,
        ttl_seconds: int = 3600,
        policy_preset: str = "sandboxed",
    ) -> None:
        self._server = server.rstrip("/")
        self._token = token
        self._workspace_id = workspace_id
        self._agent_name = agent_name
        self._agent_type = agent_type
        self._ttl_seconds = ttl_seconds
        self._policy_preset = policy_preset

        self.peer_name: str | None = None
        self.enrollment_token: str | None = None
        self._client: httpx.AsyncClient | None = None

    async def __aenter__(self) -> "LatticeAgent":
        self._client = httpx.AsyncClient(
            headers={"Authorization": f"Bearer {self._token}"},
            timeout=30,
        )
        response = await self._client.post(
            f"{self._server}/api/v1/agent-enroll",
            json={
                "agentName": self._agent_name,
                "agentType": self._agent_type,
                "workspaceId": self._workspace_id,
                "ttlSeconds": self._ttl_seconds,
                "policyPreset": self._policy_preset,
            },
        )
        response.raise_for_status()
        data = response.json()["data"]
        self.peer_name = data["peerName"]
        self.enrollment_token = data["enrollmentToken"]
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb) -> bool:
        if self._client and self.peer_name:
            try:
                await self._client.delete(
                    f"{self._server}/api/v1/agent-enroll/{self.peer_name}",
                    params={"workspaceId": self._workspace_id},
                )
            finally:
                await self._client.aclose()
        return False  # don't suppress exceptions
