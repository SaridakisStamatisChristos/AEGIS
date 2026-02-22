"""Comprehensive tests for AegisRunClient.

Uses unittest.mock to avoid hitting a real server.
"""

import json
from unittest.mock import MagicMock, PropertyMock, patch

import pytest
from requests import HTTPError, Response, Session
from requests.adapters import HTTPAdapter

from aegisrun.client import DEFAULT_TIMEOUT, AegisRunClient, _TimeoutRetryAdapter

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _mock_response(status_code: int = 200, json_data=None, content: bytes = b""):
    """Create a realistic mock Response."""
    resp = MagicMock(spec=Response)
    resp.status_code = status_code
    resp.ok = 200 <= status_code < 400
    resp.json.return_value = json_data if json_data is not None else {}
    resp.content = content
    resp.text = content.decode("utf-8", errors="replace") if content else ""
    resp.headers = {"Content-Type": "application/json"}
    resp.raise_for_status.side_effect = None if resp.ok else HTTPError(response=resp)
    # Support iter_content for streaming
    resp.iter_content.return_value = iter([content] if content else [])
    return resp


# ---------------------------------------------------------------------------
# Construction & defaults
# ---------------------------------------------------------------------------


class TestClientConstruction:

    def test_default_base_url(self):
        c = AegisRunClient()
        assert c.base_url == "http://localhost:8080"

    def test_base_url_trailing_slash_stripped(self):
        c = AegisRunClient(base_url="http://example.com/")
        assert c.base_url == "http://example.com"

    def test_api_token_stored(self):
        c = AegisRunClient(api_token="tok-abc")
        assert c.api_token == "tok-abc"

    def test_bearer_header_set_when_token_provided(self):
        c = AegisRunClient(api_token="tok-123")
        assert c.session.headers["Authorization"] == "Bearer tok-123"

    def test_no_auth_header_without_token(self):
        c = AegisRunClient()
        assert "Authorization" not in c.session.headers

    def test_custom_timeout(self):
        c = AegisRunClient(timeout=5)
        adapter = c.session.get_adapter("http://example.com")
        assert isinstance(adapter, _TimeoutRetryAdapter)
        assert adapter.timeout == 5

    def test_retry_adapter_mounted(self):
        c = AegisRunClient()
        for scheme in ("http://", "https://"):
            adapter = c.session.get_adapter(f"{scheme}example.com")
            assert isinstance(adapter, _TimeoutRetryAdapter)


# ---------------------------------------------------------------------------
# _TimeoutRetryAdapter
# ---------------------------------------------------------------------------


class TestTimeoutRetryAdapter:

    def test_default_timeout_assigned(self):
        adapter = _TimeoutRetryAdapter()
        assert adapter.timeout == DEFAULT_TIMEOUT

    def test_custom_timeout(self):
        adapter = _TimeoutRetryAdapter(timeout=10)
        assert adapter.timeout == 10


# ---------------------------------------------------------------------------
# Runs CRUD
# ---------------------------------------------------------------------------


class TestCreateRun:

    @patch.object(Session, "post")
    def test_creates_run_with_minimal_args(self, mock_post):
        mock_post.return_value = _mock_response(
            json_data={"run_id": "run-1", "status": "running"}
        )
        c = AegisRunClient()
        result = c.create_run("pol-1", "v1")

        mock_post.assert_called_once()
        call_kwargs = mock_post.call_args
        assert "/api/v1/runs/" in call_kwargs.args[0]
        body = call_kwargs.kwargs["json"]
        assert body["policy_ref"]["policy_id"] == "pol-1"
        assert body["policy_ref"]["version"] == "v1"
        assert body["metadata"] == {}
        assert result["run_id"] == "run-1"

    @patch.object(Session, "post")
    def test_creates_run_with_all_args(self, mock_post):
        mock_post.return_value = _mock_response(
            json_data={"run_id": "run-2", "status": "running"}
        )
        c = AegisRunClient()
        result = c.create_run(
            "pol-1",
            "v1",
            metadata={"env": "prod"},
            parent_run_id="parent-1",
            state_schema_ref={"schema_id": "s1", "version": "v1"},
        )

        body = mock_post.call_args.kwargs["json"]
        assert body["metadata"] == {"env": "prod"}
        assert body["parent_run_id"] == "parent-1"
        assert body["state_schema_ref"] == {"schema_id": "s1", "version": "v1"}

    @patch.object(Session, "post")
    def test_raises_on_server_error(self, mock_post):
        mock_post.return_value = _mock_response(status_code=500)
        c = AegisRunClient()
        with pytest.raises(HTTPError):
            c.create_run("pol-1", "v1")


class TestGetRun:

    @patch.object(Session, "get")
    def test_get_run_success(self, mock_get):
        mock_get.return_value = _mock_response(
            json_data={"run_id": "run-1", "status": "completed"}
        )
        c = AegisRunClient()
        result = c.get_run("run-1")

        assert "run-1" in mock_get.call_args.args[0]
        assert result["status"] == "completed"

    @patch.object(Session, "get")
    def test_get_run_not_found(self, mock_get):
        mock_get.return_value = _mock_response(status_code=404)
        c = AegisRunClient()
        with pytest.raises(HTTPError):
            c.get_run("nonexistent")


class TestListRuns:

    @patch.object(Session, "get")
    def test_list_runs_no_filters(self, mock_get):
        mock_get.return_value = _mock_response(json_data=[])
        c = AegisRunClient()
        result = c.list_runs()

        assert result == []
        params = mock_get.call_args.kwargs.get("params", {})
        assert params == {}

    @patch.object(Session, "get")
    def test_list_runs_with_filters(self, mock_get):
        mock_get.return_value = _mock_response(json_data=[{"run_id": "r1"}])
        c = AegisRunClient()
        result = c.list_runs(status="running", limit=10, offset=5)

        params = mock_get.call_args.kwargs["params"]
        assert params["status"] == "running"
        assert params["limit"] == 10
        assert params["offset"] == 5


class TestListSteps:

    @patch.object(Session, "get")
    def test_list_steps(self, mock_get):
        mock_get.return_value = _mock_response(json_data=[{"step_id": "s1"}])
        c = AegisRunClient()
        result = c.list_steps("run-1")

        assert len(result) == 1
        assert "run-1/steps" in mock_get.call_args.args[0]


class TestListEvents:

    @patch.object(Session, "get")
    def test_list_events(self, mock_get):
        mock_get.return_value = _mock_response(json_data=[{"event_id": "e1"}])
        c = AegisRunClient()
        result = c.list_events("run-1")

        assert len(result) == 1
        assert "run-1/events" in mock_get.call_args.args[0]


class TestSubmitEvent:

    @patch.object(Session, "post")
    def test_submit_event_minimal(self, mock_post):
        mock_post.return_value = _mock_response(
            json_data={"event_id": "evt-1", "event_hash": "abc123"}
        )
        c = AegisRunClient()
        result = c.submit_event("run-1", "run.started")

        body = mock_post.call_args.kwargs["json"]
        assert body["event_type"] == "run.started"
        assert "payload" not in body
        assert result["event_hash"] == "abc123"

    @patch.object(Session, "post")
    def test_submit_event_with_payload_and_timestamp(self, mock_post):
        mock_post.return_value = _mock_response(
            json_data={"event_id": "evt-2", "event_hash": "def456"}
        )
        c = AegisRunClient()
        c.submit_event(
            "run-1",
            "step.started",
            payload={"step_id": "s1"},
            timestamp="2024-01-01T00:00:00Z",
        )

        body = mock_post.call_args.kwargs["json"]
        assert body["payload"] == {"step_id": "s1"}
        assert body["timestamp"] == "2024-01-01T00:00:00Z"


# ---------------------------------------------------------------------------
# Policies CRUD
# ---------------------------------------------------------------------------


class TestPolicies:

    @patch.object(Session, "get")
    def test_list_policies(self, mock_get):
        mock_get.return_value = _mock_response(json_data=[{"policy_id": "p1"}])
        c = AegisRunClient()
        result = c.list_policies(status="deployed")

        params = mock_get.call_args.kwargs["params"]
        assert params["status"] == "deployed"
        assert len(result) == 1

    @patch.object(Session, "post")
    def test_create_policy(self, mock_post):
        mock_post.return_value = _mock_response(
            json_data={"policy_id": "p2", "name": "my-policy"}
        )
        c = AegisRunClient()
        result = c.create_policy("my-policy", spec={"rules": []})

        body = mock_post.call_args.kwargs["json"]
        assert body["name"] == "my-policy"
        assert body["spec"] == {"rules": []}

    @patch.object(Session, "get")
    def test_get_policy(self, mock_get):
        mock_get.return_value = _mock_response(
            json_data={"policy_id": "p1", "version": "v2"}
        )
        c = AegisRunClient()
        result = c.get_policy("p1", version=2)

        params = mock_get.call_args.kwargs["params"]
        assert params["version"] == 2

    @patch.object(Session, "put")
    def test_update_policy(self, mock_put):
        mock_put.return_value = _mock_response(
            json_data={"policy_id": "p1", "version": "v3"}
        )
        c = AegisRunClient()
        c.update_policy("p1", spec={"rules": [{"action": "block"}]})

        body = mock_put.call_args.kwargs["json"]
        assert body["spec"]["rules"][0]["action"] == "block"

    @patch.object(Session, "delete")
    def test_delete_policy(self, mock_del):
        mock_del.return_value = _mock_response(status_code=204)
        c = AegisRunClient()
        c.delete_policy("p1")
        assert "p1" in mock_del.call_args.args[0]

    @patch.object(Session, "post")
    def test_activate_policy(self, mock_post):
        mock_post.return_value = _mock_response(
            json_data={"policy_id": "p1", "status": "deployed"}
        )
        c = AegisRunClient()
        result = c.activate_policy("p1")
        assert "activate" in mock_post.call_args.args[0]

    @patch.object(Session, "post")
    def test_deactivate_policy(self, mock_post):
        mock_post.return_value = _mock_response(
            json_data={"policy_id": "p1", "status": "deprecated"}
        )
        c = AegisRunClient()
        result = c.deactivate_policy("p1")
        assert "deactivate" in mock_post.call_args.args[0]


# ---------------------------------------------------------------------------
# Approvals
# ---------------------------------------------------------------------------


class TestApprovals:

    @patch.object(Session, "get")
    def test_list_approvals(self, mock_get):
        mock_get.return_value = _mock_response(json_data=[])
        c = AegisRunClient()
        result = c.list_approvals(policy_id="p1", version=1)

        params = mock_get.call_args.kwargs["params"]
        assert params["policy_id"] == "p1"
        assert params["version"] == 1

    @patch.object(Session, "get")
    def test_get_approval(self, mock_get):
        mock_get.return_value = _mock_response(
            json_data={"approval_id": "a1", "decision": "approved"}
        )
        c = AegisRunClient()
        result = c.get_approval("a1")
        assert result["decision"] == "approved"

    @patch.object(Session, "post")
    def test_approve_policy(self, mock_post):
        mock_post.return_value = _mock_response(
            json_data={"approval_id": "a1", "decision": "approved"}
        )
        c = AegisRunClient()
        result = c.approve_policy("p1", version=1, comment="LGTM")

        body = mock_post.call_args.kwargs["json"]
        assert body["comment"] == "LGTM"
        assert "approve" in mock_post.call_args.args[0]

    @patch.object(Session, "post")
    def test_reject_policy(self, mock_post):
        mock_post.return_value = _mock_response(
            json_data={"approval_id": "a2", "decision": "rejected"}
        )
        c = AegisRunClient()
        result = c.reject_policy("p1", version=1, comment="Needs work")

        body = mock_post.call_args.kwargs["json"]
        assert body["comment"] == "Needs work"
        assert "reject" in mock_post.call_args.args[0]


# ---------------------------------------------------------------------------
# Gateway
# ---------------------------------------------------------------------------


class TestGateway:

    @patch.object(Session, "post")
    def test_execute_tool_call(self, mock_post):
        mock_post.return_value = _mock_response(
            json_data={
                "tool_call_id": "tc-1",
                "decision": {"action": "allow"},
                "result": {"data": "ok"},
            }
        )
        c = AegisRunClient()
        result = c.execute_tool_call(
            run_id="r1",
            step_id="s1",
            tool_name="http_request",
            args={"url": "https://example.com"},
            state_vector={"step": 1},
            executor="builtin",
        )

        body = mock_post.call_args.kwargs["json"]
        assert body["run_id"] == "r1"
        assert body["tool_name"] == "http_request"
        assert body["state_vector"] == {"step": 1}
        assert result["decision"]["action"] == "allow"

    @patch.object(Session, "post")
    def test_execute_tool_call_blocked(self, mock_post):
        mock_post.return_value = _mock_response(
            status_code=403,
            json_data={
                "tool_call_id": "tc-2",
                "decision": {"action": "block", "reason": "SSRF"},
            },
        )
        c = AegisRunClient()
        with pytest.raises(HTTPError):
            c.execute_tool_call(
                run_id="r1",
                step_id="s1",
                tool_name="http_request",
                args={"url": "http://169.254.169.254/"},
                state_vector={},
            )


# ---------------------------------------------------------------------------
# Evidence
# ---------------------------------------------------------------------------


class TestEvidence:

    @patch.object(Session, "get")
    def test_export_evidence(self, mock_get, tmp_path):
        bundle_bytes = b"PK\x03\x04fake-zip-data"
        mock_get.return_value = _mock_response(content=bundle_bytes)

        c = AegisRunClient()
        output_path = str(tmp_path / "bundle.zip")
        c.export_evidence("run-1", output_path)

        with open(output_path, "rb") as f:
            assert f.read() == bundle_bytes

    @patch.object(Session, "post")
    def test_verify_evidence(self, mock_post):
        mock_post.return_value = _mock_response(
            json_data={"valid": True, "checks": {"chain": "ok"}}
        )
        c = AegisRunClient()
        result = c.verify_evidence("run-1")
        assert result["valid"] is True


# ---------------------------------------------------------------------------
# Error handling / edge cases
# ---------------------------------------------------------------------------


class TestErrorHandling:

    @patch.object(Session, "get")
    def test_401_raises_http_error(self, mock_get):
        mock_get.return_value = _mock_response(status_code=401)
        c = AegisRunClient(api_token="expired")
        with pytest.raises(HTTPError):
            c.get_run("r1")

    @patch.object(Session, "get")
    def test_429_handled_by_retry(self, mock_get):
        """Retry strategy includes 429 in its status_forcelist."""
        c = AegisRunClient()
        adapter = c.session.get_adapter("http://example.com")
        assert 429 in adapter.max_retries.status_forcelist

    @patch.object(Session, "post")
    def test_empty_metadata_defaults(self, mock_post):
        mock_post.return_value = _mock_response(json_data={"run_id": "r1"})
        c = AegisRunClient()
        c.create_run("p1", "v1")

        body = mock_post.call_args.kwargs["json"]
        assert body["metadata"] == {}
        assert "parent_run_id" not in body
        assert "state_schema_ref" not in body
