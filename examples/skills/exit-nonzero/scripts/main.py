"""Deliberately fails for testing error handling."""
import json
import os
import sys

input_data = json.loads(os.environ.get("SANDBOX_INPUT", "{}"))

exit_code = input_data.get("exit_code", 1)
message = input_data.get("message", "deliberate failure")

# Write some stderr output before dying
print(f"exit-nonzero: about to exit with code {exit_code}", file=sys.stderr)
print(f"exit-nonzero: {message}")

# Optionally write partial output before failing
if input_data.get("write_output", False):
    output_path = os.environ.get("SANDBOX_OUTPUT", "/sandbox/out/output.json")
    os.makedirs(os.path.dirname(output_path), exist_ok=True)
    with open(output_path, "w") as f:
        json.dump({"partial": True, "message": message}, f)

sys.exit(exit_code)
