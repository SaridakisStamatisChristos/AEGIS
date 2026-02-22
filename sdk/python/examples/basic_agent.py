"""Basic agent example using AegisRun SDK"""

from aegisrun import AegisRunClient, Run


def main():
    # Initialize client
    client = AegisRunClient(
        base_url="http://localhost:8080",
        api_token=None,  # Use mock auth for local dev
    )

    # Create run
    run = Run(
        client=client,
        agent_id="demo_agent",
        policy_ref="demo-policy",
        metadata={
            "agent_name": "demo_agent",
            "environment": "development",
        },
    ).start()

    print(f"Run created: {run.run_id}")

    # Step 1: Gather information
    def gather_info(step):
        print("Step 1: Gathering information...")
        result = step.tool_call(
            tool_name="http_request",
            args={
                "url": "https://api.github.com/repos/anthropics/anthropic-sdk-python",
                "method": "GET",
            },
        )
        return result

    info = run.step(
        name="gather_information",
        state_vector={"phase": "research"},
        fn=gather_info,
    )

    print(
        f"Gathered info: {info.get('status_code') if isinstance(info, dict) else info}"
    )

    # Step 2: Process data
    def process_data(step):
        print("Step 2: Processing data...")
        # Simulate processing
        return {"processed": True, "records": 42}

    processed = run.step(
        name="process_data",
        state_vector={"phase": "processing", "data": info},
        fn=process_data,
    )

    print(f"Processed: {processed}")

    # End run
    run.end(outcome={"status": "success", "result": processed})

    print(f"Run completed successfully: {run.run_id}")
    print(f"Steps: {run.counters.steps}")
    print(f"Tool calls: {run.counters.tool_calls}")


if __name__ == "__main__":
    main()
