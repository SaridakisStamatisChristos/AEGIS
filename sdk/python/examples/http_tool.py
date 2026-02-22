"""HTTP tool usage example with error handling"""

from aegisrun import AegisRunClient, Run
from aegisrun.tool_call import ToolCallBlockedError, ToolCallExecutionError


def main():
    client = AegisRunClient(base_url="http://localhost:8080")

    run = Run(
        client=client,
        agent_id="http-demo-agent",
        policy_ref="http-demo",
        metadata={"demo": "http_tools"},
    ).start()

    print(f"Run started: {run.run_id}")

    # Allowed request
    def safe_request(step):
        print("Making allowed request...")
        try:
            result = step.tool_call(
                tool_name="http_request",
                args={
                    "url": "https://api.github.com/zen",
                    "method": "GET",
                },
            )
            print(f"Response: {result}")
            return result
        except ToolCallBlockedError as e:
            print(f"Request blocked: {e.decision.reason}")
            return None

    run.step(name="safe_request", state_vector={}, fn=safe_request)

    # Blocked request (private IP)
    def blocked_request(step):
        print("Attempting blocked request...")
        try:
            result = step.tool_call(
                tool_name="http_request",
                args={
                    "url": "http://169.254.169.254/latest/meta-data/",
                    "method": "GET",
                },
            )
            print(f"Response: {result}")
            return result
        except ToolCallBlockedError as e:
            print(f"\u2713 Request blocked as expected: {e.decision}")
            return None

    run.step(name="blocked_request", state_vector={}, fn=blocked_request)

    # Request with redaction
    def redacted_request(step):
        print("Request with sensitive data...")
        try:
            result = step.tool_call(
                tool_name="http_request",
                args={
                    "url": "https://api.example.com/webhook",
                    "method": "POST",
                    "headers": {
                        "Authorization": "Bearer secret-api-key-12345",
                    },
                },
            )
            # API key should be redacted in stored logs
            return result
        except Exception as e:
            print(f"Error: {e}")
            return None

    run.step(name="redacted_request", state_vector={}, fn=redacted_request)

    run.end(outcome={"status": "demo_complete"})

    print("\nRun completed!")
    print(f"Total tool calls: {run.counters.tool_calls}")
    print(f"Blocks: {run.counters.blocks}")

if __name__ == "__main__":
    main()
