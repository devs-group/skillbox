#!/usr/bin/env bash
# Upload all test skill ZIPs to a running skillbox server.
# Usage: ./scripts/gen-test-skills/upload-all.sh [BASE_URL] [API_KEY]
#
# Defaults:
#   BASE_URL = http://localhost:8080
#   API_KEY  = test-api-key

set -euo pipefail

BASE_URL="${1:-http://localhost:8080}"
API_KEY="${2:-test-api-key}"
DIR="$(cd "$(dirname "$0")/out" && pwd)"

echo "=== Uploading test skills to $BASE_URL ==="
echo ""

upload() {
    local file="$1"
    local expected="$2"
    local name
    name="$(basename "$file")"

    printf "%-35s -> " "$name"

    status=$(curl -s -o /tmp/skillbox-upload-response.json -w "%{http_code}" \
        -X POST "$BASE_URL/v1/skills" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/zip" \
        --data-binary "@$file" 2>/dev/null)

    if [ "$status" = "$expected" ]; then
        echo "$status ✓"
    else
        echo "$status ✗ (expected $expected)"
        cat /tmp/skillbox-upload-response.json 2>/dev/null
        echo ""
    fi
}

# Generate ZIPs if not already present
if [ ! -d "$DIR" ] || [ -z "$(ls -A "$DIR" 2>/dev/null)" ]; then
    echo "Generating test skill ZIPs..."
    (cd "$(dirname "$0")/../.." && go run ./scripts/gen-test-skills)
    echo ""
fi

echo "--- BLOCK patterns (expect 422) ---"
upload "$DIR/reverse-shell.zip"         "422"
upload "$DIR/piped-execution.zip"       "422"
upload "$DIR/crypto-miner.zip"          "422"
upload "$DIR/sandbox-escape.zip"        "422"
upload "$DIR/fork-bomb.zip"             "422"
upload "$DIR/destructive-command.zip"   "422"
upload "$DIR/base64-blob.zip"           "422"
upload "$DIR/malicious-deps-python.zip" "422"
upload "$DIR/malicious-deps-node.zip"   "422"

echo ""
echo "--- Tier 1: FLAG patterns (expect 201 - flagged but pass) ---"
upload "$DIR/eval-flag.zip"                  "201"
upload "$DIR/subprocess-flag.zip"            "201"
upload "$DIR/network-access-flag.zip"        "201"
upload "$DIR/sensitive-file-access-flag.zip" "201"

echo ""
echo "--- Tier 2: dependency deep scan (expect 422) ---"
upload "$DIR/typosquat-dep.zip"        "422"
upload "$DIR/install-hook.zip"         "422"

echo ""
echo "--- Tier 2: prompt injection (expect 422 except invisible unicode) ---"
upload "$DIR/prompt-injection.zip"     "422"
upload "$DIR/tool-call-injection.zip"  "422"
upload "$DIR/delimiter-injection.zip"  "422"
upload "$DIR/invisible-unicode.zip"    "201"

echo ""
echo "--- CLEAN (expect 201) ---"
upload "$DIR/clean-skill.zip"           "201"

echo ""
echo "=== Done ==="
