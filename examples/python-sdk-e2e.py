"""E2E tests for the Skillbox Python SDK.

Runs against a live Skillbox server (set SKILLBOX_SERVER_URL and SKILLBOX_API_KEY).
Assumes skills have already been uploaded by the test-runner service.
"""

from __future__ import annotations

import os
import sys
import tempfile
import traceback

# The SDK is mounted at /sdk/skillbox.py in the container.
from skillbox import APIError, Client, RunResult, Skill

SERVER_URL = os.environ.get("SKILLBOX_SERVER_URL", "http://localhost:8080")
API_KEY = os.environ.get("SKILLBOX_API_KEY", "sk-example-test-key")

PASS = 0
FAIL = 0


def section(name: str) -> None:
    print(f"\n\033[1;34m━━━ {name} ━━━\033[0m")


def check(name: str, passed: bool, detail: str = "") -> None:
    global PASS, FAIL
    if passed:
        PASS += 1
        print(f"  \033[32mPASS\033[0m  {name}")
    else:
        FAIL += 1
        msg = f"  \033[31mFAIL\033[0m  {name}"
        if detail:
            msg += f": {detail}"
        print(msg)


def main() -> None:
    global PASS, FAIL

    client = Client(SERVER_URL, API_KEY, timeout=120)

    # ---------------------------------------------------------------
    section("1. Health Check")
    # ---------------------------------------------------------------
    try:
        client.health()
        check("health() succeeds", True)
    except Exception as e:
        check("health() succeeds", False, str(e))

    # ---------------------------------------------------------------
    section("2. List Skills")
    # ---------------------------------------------------------------
    try:
        skills = client.list_skills()
        check("list_skills() returns list", isinstance(skills, list))
        check("list_skills() returns >0 skills", len(skills) > 0)

        if skills:
            s = skills[0]
            check("skill has name", bool(s.name))
            check("skill has version", bool(s.version))
            check("skill is Skill type", isinstance(s, Skill))

        # Check that known skills exist.
        names = {s.name for s in skills}
        check("bash-echo skill exists", "bash-echo" in names)
        check("word-counter skill exists", "word-counter" in names)
    except Exception as e:
        check("list_skills()", False, str(e))
        traceback.print_exc()

    # ---------------------------------------------------------------
    section("3. Run bash-echo Skill")
    # ---------------------------------------------------------------
    try:
        result = client.run("bash-echo", input={"message": "hello from python sdk"})
        check("run() returns RunResult", isinstance(result, RunResult))
        check("execution_id is set", bool(result.execution_id))
        check("status is success/completed", result.status in ("success", "completed"))
        check("output is not None", result.output is not None)
        check("duration_ms > 0", result.duration_ms > 0)
        check("error is empty", result.error == "")

        # Verify output contains our message.
        output = result.output
        if isinstance(output, dict):
            echo_val = output.get("echo", output.get("message", ""))
            check(
                "output contains message",
                "hello from python sdk" in str(output),
                f"output={output}",
            )
        else:
            check("output is dict", False, f"type={type(output)}")
    except Exception as e:
        check("run(bash-echo)", False, str(e))
        traceback.print_exc()

    # ---------------------------------------------------------------
    section("4. Run word-counter Skill")
    # ---------------------------------------------------------------
    try:
        result = client.run(
            "word-counter",
            input={"text": "the quick brown fox jumps over the lazy dog the"},
        )
        check("word-counter status", result.status in ("success", "completed"))
        check("word-counter has output", result.output is not None)
        if isinstance(result.output, dict):
            # Word counter should count word frequencies.
            check(
                "word-counter output has data",
                len(result.output) > 0,
                f"output={result.output}",
            )
    except Exception as e:
        check("run(word-counter)", False, str(e))
        traceback.print_exc()

    # ---------------------------------------------------------------
    section("5. Get Execution by ID")
    # ---------------------------------------------------------------
    try:
        # Use the execution_id from the previous run.
        exec_result = client.get_execution(result.execution_id)
        check("get_execution() returns RunResult", isinstance(exec_result, RunResult))
        check(
            "get_execution() matches execution_id",
            exec_result.execution_id == result.execution_id,
        )
        check(
            "get_execution() has same status",
            exec_result.status == result.status,
        )
    except Exception as e:
        check("get_execution()", False, str(e))
        traceback.print_exc()

    # ---------------------------------------------------------------
    section("6. Get Execution Logs")
    # ---------------------------------------------------------------
    try:
        logs = client.get_execution_logs(result.execution_id)
        check("get_execution_logs() returns string", isinstance(logs, str))
        # Logs may be empty for simple skills, but should not error.
        check("get_execution_logs() succeeds", True)
    except Exception as e:
        check("get_execution_logs()", False, str(e))
        traceback.print_exc()

    # ---------------------------------------------------------------
    section("7. Run with Version")
    # ---------------------------------------------------------------
    try:
        result_v = client.run("bash-echo", version="1.0.0", input={"message": "versioned"})
        check("run with version succeeds", result_v.status in ("success", "completed"))
    except APIError as e:
        # Version might not exist — 404 is acceptable.
        if e.status_code == 404:
            check("run with version (404 expected)", True)
        else:
            check("run with version", False, str(e))
    except Exception as e:
        check("run with version", False, str(e))

    # ---------------------------------------------------------------
    section("8. Run with Env")
    # ---------------------------------------------------------------
    try:
        result_env = client.run(
            "bash-echo",
            input={"message": "env-test"},
            env={"CUSTOM_VAR": "custom-value"},
        )
        check("run with env succeeds", result_env.status in ("success", "completed"))
    except Exception as e:
        check("run with env", False, str(e))

    # ---------------------------------------------------------------
    section("9. API Error Handling")
    # ---------------------------------------------------------------
    try:
        # Use an invalid API key.
        bad_client = Client(SERVER_URL, "sk-invalid-key-12345", timeout=30)
        bad_client.run("bash-echo", input={"message": "should fail"})
        check("invalid key rejected", False, "expected APIError")
    except APIError as e:
        check("invalid key rejected", e.status_code in (401, 403))
        check("APIError has status_code", e.status_code > 0)
    except Exception as e:
        check("invalid key error type", False, f"expected APIError, got {type(e).__name__}: {e}")

    try:
        # Try to run a nonexistent skill.
        client.run("nonexistent-skill-xyz")
        check("nonexistent skill rejected", False, "expected APIError")
    except APIError as e:
        check("nonexistent skill rejected", e.status_code in (404, 422, 400))
    except Exception as e:
        check("nonexistent skill error type", False, f"expected APIError, got {type(e).__name__}: {e}")

    # ---------------------------------------------------------------
    section("10. RunResult Properties")
    # ---------------------------------------------------------------
    try:
        result_files = client.run("bash-echo", input={"message": "files-check"})
        # bash-echo typically doesn't produce files.
        check("has_files is bool", isinstance(result_files.has_files, bool))
        check("files_list is list", isinstance(result_files.files_list, list))
        check("logs is str", isinstance(result_files.logs, str))
    except Exception as e:
        check("RunResult properties", False, str(e))

    # ---------------------------------------------------------------
    section("11. Download Files (if available)")
    # ---------------------------------------------------------------
    # Try to find a skill that produces files.
    try:
        skills = client.list_skills()
        has_multi_file = any(s.name == "multi-file-output" for s in skills)
        if has_multi_file:
            result_mf = client.run("multi-file-output", input={})
            if result_mf.has_files:
                with tempfile.TemporaryDirectory() as tmpdir:
                    client.download_files(result_mf, tmpdir)
                    # Check that files were extracted.
                    extracted = []
                    for root, dirs, files in os.walk(tmpdir):
                        for f in files:
                            extracted.append(os.path.relpath(os.path.join(root, f), tmpdir))
                    check("download_files() extracted files", len(extracted) > 0, f"files={extracted}")
            else:
                check("download_files() no-op (no files)", True)
        else:
            check("download_files() skipped (no multi-file-output skill)", True)
    except Exception as e:
        check("download_files()", False, str(e))
        traceback.print_exc()

    # ---------------------------------------------------------------
    # Summary
    # ---------------------------------------------------------------
    total = PASS + FAIL
    print(f"\n\033[1m{'=' * 50}\033[0m")
    print(f"Python SDK E2E: {PASS}/{total} passed", end="")
    if FAIL > 0:
        print(f", \033[31m{FAIL} FAILED\033[0m")
    else:
        print(f", \033[32mall passed\033[0m")
    print(f"\033[1m{'=' * 50}\033[0m")

    if FAIL > 0:
        sys.exit(1)


if __name__ == "__main__":
    if not API_KEY:
        print("Set SKILLBOX_API_KEY environment variable first.")
        sys.exit(1)
    main()
