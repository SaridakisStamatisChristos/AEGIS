"""API↔SDK contract tests for Python client.

These tests validate path/payload contracts against the current API surface
without requiring a live server.
"""

from unittest.mock import patch

from requests import Response, Session

from aegisrun.client import AegisRunClient
from tests.test_client import _mock_response


class TestClientContract:
    @patch.object(Session, "post")
    @patch.object(Session, "get")
    def test_run_lifecycle_routes_and_payloads(self, mock_get, mock_post):
        run_payload = {
            "run_id": "01HQXYZ123456789ABCDEFGHIJ",
            "org_id": "org-1",
            "policy_ref": {"policy_id": "policy-a", "version": "v1"},
            "metadata": {},
            "status": "running",
            "counters": {
                "steps": 0,
                "tool_calls": 0,
                "bytes_egressed": 0,
                "retries": 0,
                "blocks": 0,
            },
            "created_at": "2026-02-22T00:00:00Z",
        }

        mock_post.return_value = _mock_response(json_data=run_payload)
        mock_get.return_value = _mock_response(json_data=run_payload)

        client = AegisRunClient(base_url="http://localhost:8080", api_token="token")

        created = client.create_run("policy-a", "v1", metadata={"source": "contract-test"})
        assert mock_post.call_args.args[0].endswith("/api/v1/runs/")
        create_body = mock_post.call_args.kwargs["json"]
        assert create_body == {
            "policy_ref": {"policy_id": "policy-a", "version": "v1"},
            "metadata": {"source": "contract-test"},
        }
        assert created["run_id"] == run_payload["run_id"]

        fetched = client.get_run(run_payload["run_id"])
        assert mock_get.call_args.args[0].endswith(f"/api/v1/runs/{run_payload['run_id']}/")
        assert fetched["run_id"] == run_payload["run_id"]

    @patch.object(Session, "get")
    def test_unwraps_list_envelope_contracts(self, mock_get):
        run = {
            "run_id": "01HQXYZ123456789ABCDEFGHIJ",
            "org_id": "org-1",
            "policy_ref": {"policy_id": "policy-a", "version": "v1"},
            "metadata": {},
            "status": "running",
            "counters": {
                "steps": 0,
                "tool_calls": 0,
                "bytes_egressed": 0,
                "retries": 0,
                "blocks": 0,
            },
            "created_at": "2026-02-22T00:00:00Z",
        }
        step = {
            "step_id": "01STEP123456789ABCDEFGHIJK",
            "run_id": run["run_id"],
            "seq_no": 1,
            "name": "step-1",
            "state_vector": {},
            "status": "completed",
            "started_at": "2026-02-22T00:00:01Z",
        }
        event = {
            "event_id": "01EVT123456789ABCDEFGHIJKL",
            "run_id": run["run_id"],
            "seq_no": 1,
            "event_type": "run.started",
            "timestamp": "2026-02-22T00:00:00Z",
            "payload": {},
            "event_hash": "abc123",
        }

        mock_get.side_effect = [
            _mock_response(json_data={"runs": [run]}),
            _mock_response(json_data={"steps": [step]}),
            _mock_response(json_data={"events": [event]}),
        ]

        client = AegisRunClient()

        runs = client.list_runs(limit=10, offset=0)
        assert runs == [run]
        assert mock_get.call_args_list[0].args[0].endswith("/api/v1/runs/")

        steps = client.list_steps(run["run_id"])
        assert steps == [step]
        assert mock_get.call_args_list[1].args[0].endswith(f"/api/v1/runs/{run['run_id']}/steps")

        events = client.list_events(run["run_id"])
        assert events == [event]
        assert mock_get.call_args_list[2].args[0].endswith(f"/api/v1/runs/{run['run_id']}/events")

    @patch.object(Session, "post")
    def test_gateway_execute_route_and_decision_payload_contract(self, mock_post):
        gateway_response = {
            "tool_call_id": "01TC123456789ABCDEFGHIJKL",
            "decision": {
                "action": "block",
                "policy_rule_id": "egress.domain_allowlist",
                "reason": "domain not allowlisted",
            },
            "error": "Blocked by policy: domain not allowlisted",
        }

        mock_post.return_value = _mock_response(json_data=gateway_response)

        client = AegisRunClient()
        result = client.execute_tool_call(
            run_id="01RUN123456789ABCDEFGHIJK",
            step_id="01STEP123456789ABCDEFGHIJK",
            tool_name="http_request",
            args={"url": "https://blocked.example"},
            state_vector={"seq": 1},
        )

        assert mock_post.call_args.args[0].endswith("/api/v1/gateway/execute")
        body = mock_post.call_args.kwargs["json"]
        assert body["run_id"] == "01RUN123456789ABCDEFGHIJK"
        assert body["tool_name"] == "http_request"
        assert body["executor"] == "builtin"
        assert body["metadata"] == {}

        assert result["tool_call_id"] == gateway_response["tool_call_id"]
        assert result["decision"]["action"] == "block"
        assert result["decision"]["policy_rule_id"] == "egress.domain_allowlist"
