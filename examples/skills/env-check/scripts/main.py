"""Reports sandbox environment for testing."""
import json
import os

input_data = json.loads(os.environ.get("SANDBOX_INPUT", "{}"))

result = {
    "sandbox_input_set": "SANDBOX_INPUT" in os.environ,
    "sandbox_output_set": "SANDBOX_OUTPUT" in os.environ,
    "sandbox_files_dir_set": "SANDBOX_FILES_DIR" in os.environ,
    "skill_instructions_set": "SKILL_INSTRUCTIONS" in os.environ,
    "sandbox_output_path": os.environ.get("SANDBOX_OUTPUT", ""),
    "sandbox_files_dir_path": os.environ.get("SANDBOX_FILES_DIR", ""),
    "home": os.environ.get("HOME", ""),
    "user_id": os.getuid(),
    "working_dir": os.getcwd(),
    "custom_vars": {},
}

# Report back any custom env vars the caller asked about
check_vars = input_data.get("check_vars", [])
for var in check_vars:
    result["custom_vars"][var] = os.environ.get(var, None)

output_path = os.environ.get("SANDBOX_OUTPUT", "/sandbox/out/output.json")
os.makedirs(os.path.dirname(output_path), exist_ok=True)
with open(output_path, "w") as f:
    json.dump(result, f)
