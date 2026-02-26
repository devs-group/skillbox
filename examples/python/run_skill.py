"""Example: Using Skillbox from Python with the REST API.

No SDK required — just the requests library (or stdlib urllib).
This example uses only the standard library.
"""

import json
import os
import urllib.request
import urllib.error


BASE_URL = os.environ.get("SKILLBOX_SERVER_URL", "http://localhost:8080")
API_KEY = os.environ.get("SKILLBOX_API_KEY", "")


def api_request(method, path, body=None):
    """Make an authenticated API request to Skillbox."""
    url = f"{BASE_URL}{path}"
    data = json.dumps(body).encode() if body else None
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Authorization", f"Bearer {API_KEY}")
    if data:
        req.add_header("Content-Type", "application/json")

    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read())


def main():
    # 1. Check health
    with urllib.request.urlopen(f"{BASE_URL}/health") as resp:
        health = json.loads(resp.read())
    print(f"Server health: {health['status']}")

    # 2. List available skills
    skills = api_request("GET", "/v1/skills")
    print(f"\nAvailable skills ({len(skills)}):")
    for s in skills:
        print(f"  - {s['name']} v{s['version']}")

    # 3. Run the data-analysis skill
    print("\n--- Running data-analysis skill ---")
    result = api_request("POST", "/v1/executions", {
        "skill": "data-analysis",
        "input": {
            "data": [
                {"name": "Alice", "age": 30, "score": 85.5},
                {"name": "Bob", "age": 25, "score": 92.0},
                {"name": "Charlie", "age": 35, "score": 78.3},
                {"name": "Diana", "age": 28, "score": 95.7},
                {"name": "Eve", "age": 32, "score": 88.1},
            ]
        }
    })

    print(f"Status:       {result['status']}")
    print(f"Execution ID: {result['execution_id']}")
    print(f"Duration:     {result['duration_ms']}ms")

    if result.get("output"):
        output = result["output"]
        print(f"Rows:         {output['row_count']}")
        print(f"Columns:      {output['column_count']}")
        for col, stats in output.get("columns", {}).items():
            if stats.get("type") == "numeric":
                print(f"  {col}: mean={stats['mean']}, std={stats['std_dev']}")

    if result.get("files_list"):
        print(f"Files:        {result['files_list']}")

    # 4. Get execution logs
    print("\n--- Execution Logs ---")
    exec_id = result["execution_id"]
    log_req = urllib.request.Request(
        f"{BASE_URL}/v1/executions/{exec_id}/logs",
        method="GET",
    )
    log_req.add_header("Authorization", f"Bearer {API_KEY}")
    with urllib.request.urlopen(log_req) as resp:
        print(resp.read().decode())

    # 5. Run the text-summary skill
    print("--- Running text-summary skill ---")
    result2 = api_request("POST", "/v1/executions", {
        "skill": "text-summary",
        "input": {
            "text": (
                "Machine learning is a subset of artificial intelligence that "
                "focuses on building systems that learn from data. Unlike "
                "traditional programming where rules are explicitly coded, "
                "machine learning algorithms identify patterns in data and "
                "make decisions with minimal human intervention. Deep learning, "
                "a further subset, uses neural networks with many layers to "
                "model complex patterns in large datasets."
            ),
            "max_sentences": 2,
        }
    })

    print(f"Status:    {result2['status']}")
    if result2.get("output"):
        print(f"Summary:   {result2['output']['summary']}")
        print(f"Sentences: {result2['output']['sentence_count']}")
        print(f"Ratio:     {result2['output']['compression_ratio']}")


if __name__ == "__main__":
    if not API_KEY:
        print("Set SKILLBOX_API_KEY environment variable first.")
        print("  export SKILLBOX_API_KEY=sk-your-key")
        exit(1)
    main()
