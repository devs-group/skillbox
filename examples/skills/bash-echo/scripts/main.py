"""Echo skill: reads input and writes it back wrapped in an 'echo' field."""
import json
import os

input_data = json.loads(os.environ.get("SANDBOX_INPUT", "{}"))

result = {
    "echo": input_data,
    "runtime": "python",
    "pid": os.getpid(),
}

output_path = os.environ.get("SANDBOX_OUTPUT", "/sandbox/out/output.json")
os.makedirs(os.path.dirname(output_path), exist_ok=True)
with open(output_path, "w") as f:
    json.dump(result, f)

print("bash-echo: done")
