"""AegisRun client for API communication"""

import requests
from requests.adapters import HTTPAdapter
from urllib3.util.retry import Retry
from typing import Optional, Dict, Any, List

DEFAULT_TIMEOUT = 30  # seconds


class _TimeoutRetryAdapter(HTTPAdapter):
    """HTTPAdapter with default timeout and retry support."""

    def __init__(self, timeout: int = DEFAULT_TIMEOUT, **kwargs):
        self.timeout = timeout
        super().__init__(**kwargs)

    def send(self, *args, **kwargs):  # type: ignore[override]
        kwargs.setdefault("timeout", self.timeout)
        return super().send(*args, **kwargs)


class AegisRunClient:
    """Client for communicating with AegisRun API"""

    def __init__(
        self,
        base_url: str = "http://localhost:8080",
        api_token: Optional[str] = None,
        timeout: int = DEFAULT_TIMEOUT,
    ):
        self.base_url = base_url.rstrip("/")
        self.api_token = api_token
        self.session = requests.Session()

        # Retry on transient errors (429, 5xx) with exponential back-off
        retry_strategy = Retry(
            total=3,
            backoff_factor=0.5,
            status_forcelist=[429, 500, 502, 503, 504],
        )
        adapter = _TimeoutRetryAdapter(timeout=timeout, max_retries=retry_strategy)
        self.session.mount("http://", adapter)
        self.session.mount("https://", adapter)

        if api_token:
            self.session.headers.update({"Authorization": f"Bearer {api_token}"})

    # ── Runs ──────────────────────────────────────────────────────────

    def create_run(
        self,
        policy_id: str,
        policy_version: str,
        metadata: Optional[Dict[str, Any]] = None,
        parent_run_id: Optional[str] = None,
        state_schema_ref: Optional[Dict[str, str]] = None,
    ) -> Dict[str, Any]:
        """Create a new run"""
        payload: Dict[str, Any] = {
            "policy_ref": {
                "policy_id": policy_id,
                "version": policy_version,
            },
            "metadata": metadata or {},
        }
        if parent_run_id is not None:
            payload["parent_run_id"] = parent_run_id
        if state_schema_ref is not None:
            payload["state_schema_ref"] = state_schema_ref

        response = self.session.post(
            f"{self.base_url}/api/v1/runs/",
            json=payload,
        )
        response.raise_for_status()
        return response.json()

    def list_runs(
        self,
        status: Optional[str] = None,
        limit: Optional[int] = None,
        offset: Optional[int] = None,
    ) -> List[Dict[str, Any]]:
        """List runs with optional filters"""
        params: Dict[str, Any] = {}
        if status is not None:
            params["status"] = status
        if limit is not None:
            params["limit"] = limit
        if offset is not None:
            params["offset"] = offset

        response = self.session.get(
            f"{self.base_url}/api/v1/runs/",
            params=params,
        )
        response.raise_for_status()
        data = response.json()
        if isinstance(data, list):
            return data
        return data.get("runs", [])

    def get_run(self, run_id: str) -> Dict[str, Any]:
        """Get run details"""
        response = self.session.get(
            f"{self.base_url}/api/v1/runs/{run_id}/"
        )
        response.raise_for_status()
        return response.json()

    def list_steps(self, run_id: str) -> List[Dict[str, Any]]:
        """List steps for a run"""
        response = self.session.get(
            f"{self.base_url}/api/v1/runs/{run_id}/steps"
        )
        response.raise_for_status()
        data = response.json()
        if isinstance(data, list):
            return data
        return data.get("steps", [])

    def list_events(self, run_id: str) -> List[Dict[str, Any]]:
        """List events for a run"""
        response = self.session.get(
            f"{self.base_url}/api/v1/runs/{run_id}/events"
        )
        response.raise_for_status()
        data = response.json()
        if isinstance(data, list):
            return data
        return data.get("events", [])

    def submit_event(
        self,
        run_id: str,
        event_type: str,
        payload: Optional[Dict[str, Any]] = None,
        timestamp: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Submit a hash-chained event to a run's ledger.

        Args:
            run_id: The run to append the event to.
            event_type: One of run.started, run.ended, step.started,
                        step.ended, state.updated.
            payload: Arbitrary JSON payload for the event.
            timestamp: Optional ISO-8601 timestamp; server uses now() if omitted.

        Returns:
            The created event including its computed hash.
        """
        body: Dict[str, Any] = {"event_type": event_type}
        if payload:
            body["payload"] = payload
        if timestamp:
            body["timestamp"] = timestamp

        response = self.session.post(
            f"{self.base_url}/api/v1/runs/{run_id}/events",
            json=body,
        )
        response.raise_for_status()
        return response.json()

    # ── Policies ──────────────────────────────────────────────────────

    def list_policies(
        self, status: Optional[str] = None
    ) -> List[Dict[str, Any]]:
        """List policies"""
        params: Dict[str, Any] = {}
        if status is not None:
            params["status"] = status

        response = self.session.get(
            f"{self.base_url}/api/v1/policies/",
            params=params,
        )
        response.raise_for_status()
        return response.json()

    def create_policy(
        self,
        name: str,
        spec: Optional[Dict[str, Any]] = None,
    ) -> Dict[str, Any]:
        """Create a new policy"""
        payload = {
            "name": name,
            "spec": spec or {},
        }

        response = self.session.post(
            f"{self.base_url}/api/v1/policies/",
            json=payload,
        )
        response.raise_for_status()
        return response.json()

    def get_policy(
        self, policy_id: str, version: Optional[int] = None
    ) -> Dict[str, Any]:
        """Get a policy by ID, optionally at a specific version"""
        params: Dict[str, Any] = {}
        if version is not None:
            params["version"] = version

        response = self.session.get(
            f"{self.base_url}/api/v1/policies/{policy_id}/",
            params=params,
        )
        response.raise_for_status()
        return response.json()

    def update_policy(
        self,
        policy_id: str,
        spec: Optional[Dict[str, Any]] = None,
    ) -> Dict[str, Any]:
        """Update a policy (creates a new version)"""
        payload: Dict[str, Any] = {}
        if spec is not None:
            payload["spec"] = spec

        response = self.session.put(
            f"{self.base_url}/api/v1/policies/{policy_id}/",
            json=payload,
        )
        response.raise_for_status()
        return response.json()

    def delete_policy(self, policy_id: str) -> None:
        """Soft-delete (deprecate) a policy"""
        response = self.session.delete(
            f"{self.base_url}/api/v1/policies/{policy_id}/"
        )
        response.raise_for_status()

    def activate_policy(self, policy_id: str) -> Dict[str, Any]:
        """Deploy / activate a policy"""
        response = self.session.post(
            f"{self.base_url}/api/v1/policies/{policy_id}/activate"
        )
        response.raise_for_status()
        return response.json()

    def deactivate_policy(self, policy_id: str) -> Dict[str, Any]:
        """Undeploy / deactivate a policy"""
        response = self.session.post(
            f"{self.base_url}/api/v1/policies/{policy_id}/deactivate"
        )
        response.raise_for_status()
        return response.json()

    # ── Approvals ─────────────────────────────────────────────────────

    def list_approvals(
        self,
        policy_id: Optional[str] = None,
        version: Optional[int] = None,
    ) -> List[Dict[str, Any]]:
        """List approvals"""
        params: Dict[str, Any] = {}
        if policy_id is not None:
            params["policy_id"] = policy_id
        if version is not None:
            params["version"] = version

        response = self.session.get(
            f"{self.base_url}/api/v1/approvals/",
            params=params,
        )
        response.raise_for_status()
        return response.json()

    def get_approval(self, approval_id: str) -> Dict[str, Any]:
        """Get a single approval"""
        response = self.session.get(
            f"{self.base_url}/api/v1/approvals/{approval_id}"
        )
        response.raise_for_status()
        return response.json()

    def approve_policy(
        self,
        policy_id: str,
        version: int,
        comment: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Approve a policy version"""
        payload: Dict[str, Any] = {}
        if comment is not None:
            payload["comment"] = comment

        response = self.session.post(
            f"{self.base_url}/api/v1/approvals/policies/{policy_id}/approve",
            params={"version": version},
            json=payload,
        )
        response.raise_for_status()
        return response.json()

    def reject_policy(
        self,
        policy_id: str,
        version: int,
        comment: str,
    ) -> Dict[str, Any]:
        """Reject a policy version (comment is required)"""
        response = self.session.post(
            f"{self.base_url}/api/v1/approvals/policies/{policy_id}/reject",
            params={"version": version},
            json={"comment": comment},
        )
        response.raise_for_status()
        return response.json()

    # ── Gateway ───────────────────────────────────────────────────────

    def execute_tool_call(
        self,
        run_id: str,
        step_id: str,
        tool_name: str,
        args: Dict[str, Any],
        state_vector: Dict[str, Any],
        metadata: Optional[Dict[str, Any]] = None,
        executor: str = "builtin",
    ) -> Dict[str, Any]:
        """Execute a tool call through the gateway"""
        payload = {
            "run_id": run_id,
            "step_id": step_id,
            "tool_name": tool_name,
            "args": args,
            "executor": executor,
            "state_vector": state_vector,
            "metadata": metadata or {},
        }

        response = self.session.post(
            f"{self.base_url}/api/v1/gateway/execute",
            json=payload,
        )
        response.raise_for_status()
        return response.json()

    # ── Evidence ──────────────────────────────────────────────────────

    def export_evidence(self, run_id: str, output_path: str):
        """Export evidence bundle to file"""
        response = self.session.get(
            f"{self.base_url}/api/v1/evidence/runs/{run_id}/bundle",
            stream=True,
        )
        response.raise_for_status()

        with open(output_path, "wb") as f:
            for chunk in response.iter_content(chunk_size=8192):
                f.write(chunk)

    def verify_evidence(self, run_id: str) -> Dict[str, Any]:
        """Verify evidence chain for a run"""
        response = self.session.post(
            f"{self.base_url}/api/v1/evidence/verify",
            json={"run_id": run_id},
        )
        response.raise_for_status()
        return response.json()
