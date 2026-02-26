"""Sleeps to test timeout enforcement."""
import json
import os
import time

input_data = json.loads(os.environ.get("SANDBOX_INPUT", "{}"))

sleep_seconds = input_data.get("sleep_seconds", 30)
print(f"slow-skill: sleeping for {sleep_seconds}s")
time.sleep(sleep_seconds)

# If we get here, timeout didn't fire
result = {"completed": True, "slept": sleep_seconds}

output_path = os.environ.get("SANDBOX_OUTPUT", "/sandbox/out/output.json")
os.makedirs(os.path.dirname(output_path), exist_ok=True)
with open(output_path, "w") as f:
    json.dump(result, f)

print("slow-skill: done (not timed out)")
