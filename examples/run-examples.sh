#!/bin/sh
set -e

API_URL="http://api:8080"
API_KEY="sk-example-test-key"

FAILURES=0
TESTS=0

pass() { printf "  \033[32mPASS\033[0m  %s\n" "$1"; }
fail() { printf "  \033[31mFAIL\033[0m  %s: %s\n" "$1" "$2"; FAILURES=$((FAILURES + 1)); }
section() { printf "\n\033[1;34m━━━ %s ━━━\033[0m\n" "$1"; }

check() {
    TESTS=$((TESTS + 1))
    NAME="$1"
    EXPECTED="$2"
    ACTUAL="$3"
    if [ "$EXPECTED" = "$ACTUAL" ]; then
        pass "$NAME"
    else
        fail "$NAME" "expected '$EXPECTED', got '$ACTUAL'"
    fi
}

check_contains() {
    TESTS=$((TESTS + 1))
    NAME="$1"
    HAYSTACK="$2"
    NEEDLE="$3"
    if echo "$HAYSTACK" | grep -q "$NEEDLE"; then
        pass "$NAME"
    else
        fail "$NAME" "expected to contain '$NEEDLE', got: $(echo "$HAYSTACK" | head -c 200)"
    fi
}

check_not_contains() {
    TESTS=$((TESTS + 1))
    NAME="$1"
    HAYSTACK="$2"
    NEEDLE="$3"
    if echo "$HAYSTACK" | grep -q "$NEEDLE"; then
        fail "$NAME" "should NOT contain '$NEEDLE'"
    else
        pass "$NAME"
    fi
}

http_code() {
    curl -s -o /dev/null -w '%{http_code}' "$@" 2>/dev/null || echo "000"
}

http_body_and_code() {
    # Writes body to stdout, code to fd 3
    TMPFILE="/tmp/http_response_$$"
    CODE=$(curl -s -o "$TMPFILE" -w '%{http_code}' "$@" 2>/dev/null || echo "000")
    cat "$TMPFILE"
    rm -f "$TMPFILE"
    # Store code in a global for the caller
    LAST_HTTP_CODE="$CODE"
}

upload_skill() {
    SKILL_DIR="$1"
    SKILL_NAME="$2"
    VERSION="$3"

    ZIPFILE="/tmp/${SKILL_NAME}.zip"
    rm -f "$ZIPFILE"
    (cd "$SKILL_DIR" && zip -r "$ZIPFILE" . > /dev/null 2>&1)

    TESTS=$((TESTS + 1))
    CODE=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API_URL/v1/skills" \
        -H "Authorization: Bearer $API_KEY" \
        -F "file=@$ZIPFILE" \
        -F "name=$SKILL_NAME" \
        -F "version=$VERSION" 2>/dev/null)

    if [ "$CODE" = "201" ] || [ "$CODE" = "200" ]; then
        pass "Upload $SKILL_NAME v$VERSION"
    else
        fail "Upload $SKILL_NAME v$VERSION" "HTTP $CODE"
    fi
}

run_skill() {
    BODY="$1"
    curl -s -X POST "$API_URL/v1/executions" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        -d "$BODY" 2>/dev/null
}

extract_field() {
    # Poor man's JSON field extraction (no jq available)
    echo "$1" | grep -o "\"$2\":[^,}]*" | head -1 | sed "s/\"$2\"://" | tr -d '"' | tr -d ' '
}

# ###############################################
section "Waiting for API"
# ###############################################
for i in $(seq 1 30); do
    if curl -sf "$API_URL/health" > /dev/null 2>&1; then
        echo "API is ready (attempt $i)"
        break
    fi
    if [ "$i" = "30" ]; then
        echo "API did not become ready in time"
        exit 1
    fi
    sleep 2
done

# ###############################################
section "1. Health & Readiness"
# ###############################################
HEALTH=$(curl -sf "$API_URL/health")
check_contains "GET /health returns ok" "$HEALTH" '"ok"'

READY=$(curl -sf "$API_URL/ready")
check_contains "GET /ready returns ready" "$READY" '"ready"'
check_contains "GET /ready has postgres check" "$READY" '"postgres"'

# Health endpoints require no auth
check "GET /health needs no auth" "200" "$(http_code "$API_URL/health")"
check "GET /ready needs no auth" "200" "$(http_code "$API_URL/ready")"

# ###############################################
section "2. Authentication & Authorization"
# ###############################################
check "No auth header → 401" "401" "$(http_code "$API_URL/v1/skills")"
check "Empty Bearer → 401" "401" "$(http_code -H 'Authorization: Bearer ' "$API_URL/v1/skills")"
check "Wrong key → 401" "401" "$(http_code -H 'Authorization: Bearer wrong-key-12345' "$API_URL/v1/skills")"
check "Basic auth scheme → 401" "401" "$(http_code -H 'Authorization: Basic dXNlcjpwYXNz' "$API_URL/v1/skills")"
check "Malformed header (no space) → 401" "401" "$(http_code -H 'Authorization: Bearertoken' "$API_URL/v1/skills")"
check "Valid key → 200" "200" "$(http_code -H "Authorization: Bearer $API_KEY" "$API_URL/v1/skills")"

# Auth on other endpoints
check "POST /v1/executions without auth → 401" "401" "$(http_code -X POST "$API_URL/v1/executions")"
check "GET /v1/executions/:id without auth → 401" "401" "$(http_code "$API_URL/v1/executions/nonexistent-id")"
check "DELETE /v1/skills/:n/:v without auth → 401" "401" "$(http_code -X DELETE "$API_URL/v1/skills/x/1.0.0")"

# ###############################################
section "3. Skill Upload — happy paths"
# ###############################################
upload_skill "/skills/data-analysis" "data-analysis" "1.0.0"
upload_skill "/skills/text-summary" "text-summary" "1.0.0"
upload_skill "/skills/word-counter" "word-counter" "1.0.0"
upload_skill "/skills/bash-echo" "bash-echo" "1.0.0"
upload_skill "/skills/env-check" "env-check" "1.0.0"
upload_skill "/skills/multi-file-output" "multi-file-output" "1.0.0"
upload_skill "/skills/exit-nonzero" "exit-nonzero" "1.0.0"
upload_skill "/skills/slow-skill" "slow-skill" "1.0.0"

# Upload a second version of data-analysis by patching SKILL.md in a temp copy
mkdir -p /tmp/data-analysis-v2
cp -r /skills/data-analysis/* /tmp/data-analysis-v2/
sed -i 's/version: "1.0.0"/version: "2.0.0"/' /tmp/data-analysis-v2/SKILL.md
upload_skill "/tmp/data-analysis-v2" "data-analysis" "2.0.0"

# ###############################################
section "4. Skill Upload — error cases"
# ###############################################

# Upload with wrong content type
TESTS=$((TESTS + 1))
CODE=$(http_code -X POST "$API_URL/v1/skills" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{"not": "a zip"}')
check "Upload with JSON content-type → 415" "415" "$CODE"

# Upload empty body as zip
TESTS=$((TESTS + 1))
CODE=$(http_code -X POST "$API_URL/v1/skills" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/zip")
check "Upload empty zip body → 400" "400" "$CODE"

# Upload invalid zip (just random bytes)
echo "this is not a zip file" > /tmp/notazip.zip
TESTS=$((TESTS + 1))
CODE=$(http_code -X POST "$API_URL/v1/skills" \
    -H "Authorization: Bearer $API_KEY" \
    -F "file=@/tmp/notazip.zip")
check "Upload invalid zip → 400" "400" "$CODE"

# Upload valid zip without SKILL.md
mkdir -p /tmp/noskill && echo "hello" > /tmp/noskill/readme.txt
(cd /tmp/noskill && zip -r /tmp/noskillmd.zip . > /dev/null 2>&1)
TESTS=$((TESTS + 1))
CODE=$(http_code -X POST "$API_URL/v1/skills" \
    -H "Authorization: Bearer $API_KEY" \
    -F "file=@/tmp/noskillmd.zip")
check "Upload zip without SKILL.md → 400" "400" "$CODE"

# Upload zip with invalid SKILL.md (bad YAML)
mkdir -p /tmp/badskill
printf "---\nname: [invalid yaml\n---\n" > /tmp/badskill/SKILL.md
(cd /tmp/badskill && zip -r /tmp/badskill.zip . > /dev/null 2>&1)
TESTS=$((TESTS + 1))
CODE=$(http_code -X POST "$API_URL/v1/skills" \
    -H "Authorization: Bearer $API_KEY" \
    -F "file=@/tmp/badskill.zip")
check "Upload zip with invalid YAML → 400" "400" "$CODE"

# Upload zip with missing required fields in SKILL.md
mkdir -p /tmp/incompleteskill
printf "---\nname: incomplete\n---\n" > /tmp/incompleteskill/SKILL.md
(cd /tmp/incompleteskill && zip -r /tmp/incomplete.zip . > /dev/null 2>&1)
TESTS=$((TESTS + 1))
CODE=$(http_code -X POST "$API_URL/v1/skills" \
    -H "Authorization: Bearer $API_KEY" \
    -F "file=@/tmp/incomplete.zip")
check "Upload zip with missing fields → 400" "400" "$CODE"

# ###############################################
section "5. Skill Listing & Metadata"
# ###############################################
SKILLS=$(curl -sf "$API_URL/v1/skills" -H "Authorization: Bearer $API_KEY")
SKILL_COUNT=$(echo "$SKILLS" | grep -o '"name"' | wc -l | tr -d ' ')

TESTS=$((TESTS + 1))
if [ "$SKILL_COUNT" -ge 8 ]; then
    pass "List returns >= 8 skills (got $SKILL_COUNT)"
else
    fail "List skill count" "expected >= 8, got $SKILL_COUNT"
fi

check_contains "List contains data-analysis" "$SKILLS" '"data-analysis"'
check_contains "List contains bash-echo" "$SKILLS" '"bash-echo"'
check_contains "List contains env-check" "$SKILLS" '"env-check"'

# Get specific skill metadata
META=$(curl -sf "$API_URL/v1/skills/data-analysis/1.0.0" -H "Authorization: Bearer $API_KEY")
check_contains "GET skill meta: name" "$META" '"data-analysis"'
check_contains "GET skill meta: version" "$META" '"1.0.0"'
check_contains "GET skill meta: lang=python" "$META" '"python"'

# Get non-existent skill → 404
check "GET non-existent skill → 404" "404" "$(http_code -H "Authorization: Bearer $API_KEY" "$API_URL/v1/skills/does-not-exist/9.9.9")"

# ###############################################
section "6. Execution — data-analysis (JSON)"
# ###############################################
RESULT=$(run_skill '{
    "skill": "data-analysis",
    "version": "1.0.0",
    "input": {
        "data": [
            {"name": "Alice", "age": 30, "score": 85.5},
            {"name": "Bob", "age": 25, "score": 92.0},
            {"name": "Charlie", "age": 35, "score": 78.3}
        ]
    }
}')

check_contains "data-analysis JSON: status=success" "$RESULT" '"status":"success"'
check_contains "data-analysis: has row_count" "$RESULT" '"row_count"'
check_contains "data-analysis: has column_count" "$RESULT" '"column_count"'
check_contains "data-analysis: has columns" "$RESULT" '"columns"'
check_contains "data-analysis: has numeric_columns" "$RESULT" '"numeric_columns"'
check_contains "data-analysis: has file artifacts" "$RESULT" '"files_list"'
check_contains "data-analysis: has execution_id" "$RESULT" '"execution_id"'
check_contains "data-analysis: has duration_ms" "$RESULT" '"duration_ms"'
check_contains "data-analysis: summary.txt artifact" "$RESULT" 'summary.txt'

# ###############################################
section "7. Execution — data-analysis (CSV)"
# ###############################################
RESULT=$(run_skill '{"skill": "data-analysis", "version": "1.0.0", "input": {"csv": "name,age,salary\nAlice,30,75000\nBob,25,65000\nCharlie,35,95000"}}')
check_contains "data-analysis CSV: status=success" "$RESULT" '"status":"success"'
check_contains "data-analysis CSV: row_count=3" "$RESULT" '"row_count":3'

# ###############################################
section "8. Execution — latest version resolution"
# ###############################################
# We uploaded data-analysis v1.0.0 and v2.0.0 — omitting version should resolve to latest
RESULT=$(run_skill '{"skill": "data-analysis", "input": {"data": [{"x": 1}, {"x": 2}]}}')
check_contains "latest version: status=success" "$RESULT" '"status":"success"'

# Explicitly request "latest"
RESULT=$(run_skill '{"skill": "data-analysis", "version": "latest", "input": {"data": [{"x": 10}]}}')
check_contains "explicit latest: status=success" "$RESULT" '"status":"success"'

# ###############################################
section "9. Execution — text-summary"
# ###############################################
RESULT=$(run_skill '{
    "skill": "text-summary",
    "version": "1.0.0",
    "input": {
        "text": "Artificial intelligence is intelligence demonstrated by machines. AI research has been defined as the field of study of intelligent agents. The term artificial intelligence had previously been used to describe machines that mimic human cognitive skills. This definition has since been rejected by major AI researchers. Modern AI techniques include machine learning, deep learning, and natural language processing. These methods have achieved remarkable results in image recognition, language translation, and game playing.",
        "max_sentences": 2
    }
}')

check_contains "text-summary: status=success" "$RESULT" '"status":"success"'
check_contains "text-summary: has summary" "$RESULT" '"summary"'
check_contains "text-summary: has sentence_count" "$RESULT" '"sentence_count"'
check_contains "text-summary: has compression_ratio" "$RESULT" '"compression_ratio"'
check_contains "text-summary: has original_length" "$RESULT" '"original_length"'

# Test text-summary with empty text
RESULT=$(run_skill '{"skill": "text-summary", "version": "1.0.0", "input": {"text": ""}}')
check_contains "text-summary empty text: success" "$RESULT" '"status":"success"'
check_contains "text-summary empty text: error field" "$RESULT" '"No text provided"'

# ###############################################
section "10. Execution — word-counter"
# ###############################################
RESULT=$(run_skill '{"skill": "word-counter", "version": "1.0.0", "input": {"text": "the quick brown fox jumps over the lazy dog the fox the", "top_n": 3}}')

check_contains "word-counter: status=success" "$RESULT" '"status":"success"'
check_contains "word-counter: has total_words" "$RESULT" '"total_words"'
check_contains "word-counter: has unique_words" "$RESULT" '"unique_words"'
check_contains "word-counter: has top_words" "$RESULT" '"top_words"'
check_contains "word-counter: has report.txt artifact" "$RESULT" 'report.txt'

# Verify exact word count: "the quick brown fox jumps over the lazy dog the fox the" = 12 words
check_contains "word-counter: total_words=12" "$RESULT" '"total_words":12'

# ###############################################
section "11. Execution — echo (input/output round-trip)"
# ###############################################
RESULT=$(run_skill '{"skill": "bash-echo", "version": "1.0.0", "input": {"greeting": "hello", "number": 42}}')

check_contains "bash-echo: status=success" "$RESULT" '"status":"success"'
check_contains "bash-echo: echoed greeting" "$RESULT" '"greeting"'
check_contains "bash-echo: echoed number" "$RESULT" '"number"'
check_contains "bash-echo: reports runtime" "$RESULT" '"runtime"'

# ###############################################
section "12. Execution — env-check (sandbox environment)"
# ###############################################
RESULT=$(run_skill '{"skill": "env-check", "version": "1.0.0", "input": {"check_vars": ["HOME", "PATH"]}}')

check_contains "env-check: status=success" "$RESULT" '"status":"success"'
check_contains "env-check: SANDBOX_INPUT set" "$RESULT" '"sandbox_input_set":true'
check_contains "env-check: SANDBOX_OUTPUT set" "$RESULT" '"sandbox_output_set":true'
check_contains "env-check: SANDBOX_FILES_DIR set" "$RESULT" '"sandbox_files_dir_set":true'
check_contains "env-check: SKILL_INSTRUCTIONS set" "$RESULT" '"skill_instructions_set":true'

# Verify sandbox paths are correct
check_contains "env-check: output path" "$RESULT" '/sandbox/out/output.json'
check_contains "env-check: files dir path" "$RESULT" '/sandbox/out/files'

# ###############################################
section "13. Execution — custom env vars"
# ###############################################
RESULT=$(run_skill '{"skill": "env-check", "version": "1.0.0", "input": {"check_vars": ["MY_CUSTOM_VAR", "ANOTHER_VAR"]}, "env": {"MY_CUSTOM_VAR": "test-value-123", "ANOTHER_VAR": "second-value"}}')

check_contains "custom env: status=success" "$RESULT" '"status":"success"'
check_contains "custom env: MY_CUSTOM_VAR passed" "$RESULT" 'test-value-123'
check_contains "custom env: ANOTHER_VAR passed" "$RESULT" 'second-value'

# ###############################################
section "14. Execution — multi-file-output (artifact collection)"
# ###############################################
RESULT=$(run_skill '{"skill": "multi-file-output", "version": "1.0.0", "input": {"file_count": 5}}')

check_contains "multi-file: status=success" "$RESULT" '"status":"success"'
check_contains "multi-file: has files_list" "$RESULT" '"files_list"'
check_contains "multi-file: has files_url" "$RESULT" '"files_url"'
check_contains "multi-file: has output_0.txt" "$RESULT" 'output_0.txt'
check_contains "multi-file: has output_4.txt" "$RESULT" 'output_4.txt'
check_contains "multi-file: has data.csv" "$RESULT" 'data.csv'
check_contains "multi-file: has nested/deep.txt" "$RESULT" 'nested/deep.txt'

# Verify files_url is a presigned S3 URL
check_contains "multi-file: files_url is http" "$RESULT" 'http'

# ###############################################
section "15. Execution — failure handling (non-zero exit)"
# ###############################################
RESULT=$(run_skill '{"skill": "exit-nonzero", "version": "1.0.0", "input": {"exit_code": 42, "message": "boom"}}')

check_contains "exit-nonzero: status=failed" "$RESULT" '"status":"failed"'
check_contains "exit-nonzero: has error field" "$RESULT" '"error"'
check_contains "exit-nonzero: has logs" "$RESULT" '"logs"'
check_contains "exit-nonzero: stderr captured" "$RESULT" 'exit-nonzero'

# ###############################################
section "16. Execution — timeout enforcement"
# ###############################################
# slow-skill has timeout: 5s but we ask it to sleep 30s
RESULT=$(run_skill '{"skill": "slow-skill", "version": "1.0.0", "input": {"sleep_seconds": 30}}')

# Should get a timeout error (either in result or HTTP status)
TESTS=$((TESTS + 1))
if echo "$RESULT" | grep -q '"timeout"' || echo "$RESULT" | grep -q '"timed out"' || echo "$RESULT" | grep -q 'timeout'; then
    pass "slow-skill: timed out as expected"
else
    fail "slow-skill timeout" "expected timeout, got: $(echo "$RESULT" | head -c 300)"
fi

# ###############################################
section "17. Execution — error cases"
# ###############################################

# Missing skill field
TESTS=$((TESTS + 1))
CODE=$(http_code -X POST "$API_URL/v1/executions" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{"input": {"data": [1,2,3]}}')
check "Execute without skill field → 400" "400" "$CODE"

# Non-existent skill
TESTS=$((TESTS + 1))
CODE=$(http_code -X POST "$API_URL/v1/executions" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{"skill": "skill-that-does-not-exist"}')
check "Execute non-existent skill → 404" "404" "$CODE"

# Invalid JSON body
TESTS=$((TESTS + 1))
CODE=$(http_code -X POST "$API_URL/v1/executions" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d 'this is not json')
check "Execute with invalid JSON → 400" "400" "$CODE"

# Empty body
TESTS=$((TESTS + 1))
CODE=$(http_code -X POST "$API_URL/v1/executions" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d '')
check "Execute with empty body → 400" "400" "$CODE"

# Non-existent version — execution is created but fails during skill loading
RESULT=$(run_skill '{"skill": "data-analysis", "version": "99.99.99", "input": {}}')
check_contains "Execute non-existent version: failed" "$RESULT" '"status":"failed"'
check_contains "Execute non-existent version: has error" "$RESULT" '"error"'

# ###############################################
section "18. Execution — null and empty input"
# ###############################################

# Null input should default to {}
RESULT=$(run_skill '{"skill": "word-counter", "version": "1.0.0"}')
check_contains "null input: status=success" "$RESULT" '"status":"success"'
check_contains "null input: total_words=0" "$RESULT" '"total_words":0'

# Empty object input
RESULT=$(run_skill '{"skill": "word-counter", "version": "1.0.0", "input": {}}')
check_contains "empty input: status=success" "$RESULT" '"status":"success"'
check_contains "empty input: total_words=0" "$RESULT" '"total_words":0'

# ###############################################
section "19. Execution retrieval & logs"
# ###############################################

# Run a skill and capture its execution ID
RESULT=$(run_skill '{"skill": "word-counter", "version": "1.0.0", "input": {"text": "hello world"}}')
EXEC_ID=$(echo "$RESULT" | grep -o '"execution_id":"[^"]*"' | head -1 | sed 's/"execution_id":"//;s/"//')

TESTS=$((TESTS + 1))
if [ -n "$EXEC_ID" ]; then
    pass "Execution ID extracted: $EXEC_ID"
else
    fail "Execution ID extraction" "could not extract from response"
fi

if [ -n "$EXEC_ID" ]; then
    # GET /v1/executions/:id
    EXEC_RESULT=$(curl -sf "$API_URL/v1/executions/$EXEC_ID" -H "Authorization: Bearer $API_KEY")
    check_contains "GET execution: has status" "$EXEC_RESULT" '"status"'
    check_contains "GET execution: has execution_id" "$EXEC_RESULT" "$EXEC_ID"

    # GET /v1/executions/:id/logs
    LOGS=$(curl -sf "$API_URL/v1/executions/$EXEC_ID/logs" -H "Authorization: Bearer $API_KEY")
    TESTS=$((TESTS + 1))
    if [ -n "$LOGS" ]; then
        pass "GET execution logs: non-empty"
    else
        fail "GET execution logs" "empty response"
    fi
    check_contains "Logs contain skill output" "$LOGS" 'Counted'

    # Non-existent execution ID → 404
    check "GET non-existent execution → 404" "404" "$(http_code -H "Authorization: Bearer $API_KEY" "$API_URL/v1/executions/00000000-0000-0000-0000-000000000000")"

    # Logs for non-existent execution → 404
    check "GET logs for non-existent → 404" "404" "$(http_code -H "Authorization: Bearer $API_KEY" "$API_URL/v1/executions/00000000-0000-0000-0000-000000000000/logs")"
fi

# ###############################################
section "20. Skill versioning"
# ###############################################

# We uploaded data-analysis v1.0.0 and v2.0.0
# Both should be accessible
check "GET data-analysis v1.0.0 → 200" "200" "$(http_code -H "Authorization: Bearer $API_KEY" "$API_URL/v1/skills/data-analysis/1.0.0")"
check "GET data-analysis v2.0.0 → 200" "200" "$(http_code -H "Authorization: Bearer $API_KEY" "$API_URL/v1/skills/data-analysis/2.0.0")"

# Can execute specific version
RESULT=$(run_skill '{"skill": "data-analysis", "version": "1.0.0", "input": {"data": [{"v": 1}]}}')
check_contains "Execute v1.0.0: success" "$RESULT" '"status":"success"'

RESULT=$(run_skill '{"skill": "data-analysis", "version": "2.0.0", "input": {"data": [{"v": 2}]}}')
check_contains "Execute v2.0.0: success" "$RESULT" '"status":"success"'

# ###############################################
section "21. Skill deletion"
# ###############################################

# Delete word-counter v1.0.0
check "DELETE word-counter → 204" "204" "$(http_code -X DELETE -H "Authorization: Bearer $API_KEY" "$API_URL/v1/skills/word-counter/1.0.0")"

# Verify it's gone from listing
SKILLS=$(curl -sf "$API_URL/v1/skills" -H "Authorization: Bearer $API_KEY")
check_not_contains "word-counter removed from list" "$SKILLS" '"word-counter"'

# Verify GET returns 404
check "GET deleted skill → 404" "404" "$(http_code -H "Authorization: Bearer $API_KEY" "$API_URL/v1/skills/word-counter/1.0.0")"

# Verify execution of deleted skill fails (specific version → status=failed)
RESULT=$(run_skill '{"skill": "word-counter", "version": "1.0.0", "input": {"text": "test"}}')
check_contains "Execute deleted skill (specific version): failed" "$RESULT" '"status":"failed"'

# Verify execution of deleted skill with latest resolution → 404
check "Execute deleted skill (latest) → 404" "404" "$(http_code -X POST "$API_URL/v1/executions" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{"skill": "word-counter", "input": {"text": "test"}}')"

# Delete non-existent skill → 404
check "DELETE non-existent → 404" "404" "$(http_code -X DELETE -H "Authorization: Bearer $API_KEY" "$API_URL/v1/skills/nonexistent/1.0.0")"

# Delete one version doesn't affect others
check "DELETE data-analysis v1 → 204" "204" "$(http_code -X DELETE -H "Authorization: Bearer $API_KEY" "$API_URL/v1/skills/data-analysis/1.0.0")"
check "data-analysis v2 still exists → 200" "200" "$(http_code -H "Authorization: Bearer $API_KEY" "$API_URL/v1/skills/data-analysis/2.0.0")"

# ###############################################
section "22. Concurrent executions"
# ###############################################

# Fire 3 executions in parallel, collect PIDs
TMPDIR_CONC="/tmp/concurrent_$$"
mkdir -p "$TMPDIR_CONC"

for i in 1 2 3; do
    curl -s -X POST "$API_URL/v1/executions" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        -d "{\"skill\": \"env-check\", \"version\": \"1.0.0\", \"input\": {\"check_vars\": [\"PARALLEL_ID\"]}, \"env\": {\"PARALLEL_ID\": \"run-$i\"}}" \
        > "$TMPDIR_CONC/result_$i.json" 2>/dev/null &
done
wait

# Verify all 3 succeeded
CONC_PASS=0
for i in 1 2 3; do
    if grep -q '"status":"success"' "$TMPDIR_CONC/result_$i.json" 2>/dev/null; then
        CONC_PASS=$((CONC_PASS + 1))
    fi
done

check "Concurrent: all 3 succeeded" "3" "$CONC_PASS"

# Verify each got its own PARALLEL_ID
for i in 1 2 3; do
    TESTS=$((TESTS + 1))
    if grep -q "run-$i" "$TMPDIR_CONC/result_$i.json" 2>/dev/null; then
        pass "Concurrent: run-$i got correct env"
    else
        fail "Concurrent: run-$i env" "PARALLEL_ID mismatch"
    fi
done

# Each should have a unique execution_id
ID1=$(grep -o '"execution_id":"[^"]*"' "$TMPDIR_CONC/result_1.json" | head -1)
ID2=$(grep -o '"execution_id":"[^"]*"' "$TMPDIR_CONC/result_2.json" | head -1)
ID3=$(grep -o '"execution_id":"[^"]*"' "$TMPDIR_CONC/result_3.json" | head -1)

TESTS=$((TESTS + 1))
if [ "$ID1" != "$ID2" ] && [ "$ID2" != "$ID3" ] && [ "$ID1" != "$ID3" ]; then
    pass "Concurrent: all execution IDs are unique"
else
    fail "Concurrent: unique IDs" "got $ID1, $ID2, $ID3"
fi

rm -rf "$TMPDIR_CONC"

# ###############################################
section "23. Re-upload after delete"
# ###############################################

# Re-upload word-counter (was deleted in section 21)
upload_skill "/skills/word-counter" "word-counter" "1.0.0"

# Execute it to prove it works after re-upload
RESULT=$(run_skill '{"skill": "word-counter", "version": "1.0.0", "input": {"text": "upload works again"}}')
check_contains "Re-uploaded skill: success" "$RESULT" '"status":"success"'
check_contains "Re-uploaded skill: counted words" "$RESULT" '"total_words":3'

# ###############################################
section "24. Large input handling"
# ###############################################

# Re-upload data-analysis for this section (v1 was deleted in section 21)
upload_skill "/skills/data-analysis" "data-analysis" "1.0.0"

# Generate a large-ish input (many records)
LARGE_INPUT='{"skill": "data-analysis", "version": "1.0.0", "input": {"data": ['
for i in $(seq 1 100); do
    if [ "$i" -gt 1 ]; then LARGE_INPUT="$LARGE_INPUT,"; fi
    LARGE_INPUT="$LARGE_INPUT{\"id\":$i,\"value\":$((i * 7)),\"name\":\"record_$i\"}"
done
LARGE_INPUT="$LARGE_INPUT]}}"

RESULT=$(run_skill "$LARGE_INPUT")
check_contains "Large input (100 records): success" "$RESULT" '"status":"success"'
check_contains "Large input: row_count=100" "$RESULT" '"row_count":100'

# ###############################################
section "25. Edge case: special characters in input"
# ###############################################

RESULT=$(run_skill '{"skill": "word-counter", "version": "1.0.0", "input": {"text": "hello \"world\" & <script>alert(1)</script> newline\nhere tab\there"}}')
check_contains "Special chars: success" "$RESULT" '"status":"success"'

# Unicode input
RESULT=$(run_skill '{"skill": "text-summary", "version": "1.0.0", "input": {"text": "The caf\u00e9 serves pi\u00f1a coladas. The na\u00efve r\u00e9sum\u00e9 was accepted."}}')
check_contains "Unicode input: success" "$RESULT" '"status":"success"'

# ###############################################
section "26. API error response format"
# ###############################################

# Verify error responses have the correct structure
ERROR_RESP=$(curl -s "$API_URL/v1/skills" 2>/dev/null)
check_contains "Error response has 'error' field" "$ERROR_RESP" '"error"'
check_contains "Error response has 'message' field" "$ERROR_RESP" '"message"'

# ╔══════════════════════════════════════════════╗
# ║         PERFORMANCE BENCHMARKS               ║
# ╚══════════════════════════════════════════════╝

# Helpers for performance tests.
# curl_ms: returns HTTP request time in milliseconds (integer).
curl_ms() {
    # %{time_total} returns seconds with 6 decimal places (e.g. "0.012345").
    # We multiply by 1000 and truncate to get integer ms.
    RAW=$(curl -s -o /dev/null -w '%{time_total}' "$@" 2>/dev/null || echo "99")
    # Shell can't do float math — strip the dot and do integer arithmetic.
    # "0.012345" → whole=0, frac=012 → ms = 0*1000 + 12 = 12
    WHOLE=$(echo "$RAW" | cut -d. -f1)
    FRAC=$(echo "$RAW" | cut -d. -f2 | cut -c1-3)
    # Pad fraction to 3 digits if needed
    while [ ${#FRAC} -lt 3 ]; do FRAC="${FRAC}0"; done
    # Remove leading zeros for arithmetic
    FRAC=$(echo "$FRAC" | sed 's/^0*//' )
    [ -z "$FRAC" ] && FRAC=0
    echo $(( WHOLE * 1000 + FRAC ))
}

# check_under_ms: PASS if actual_ms <= threshold_ms.
check_under_ms() {
    TESTS=$((TESTS + 1))
    NAME="$1"
    ACTUAL_MS="$2"
    THRESHOLD_MS="$3"
    if [ "$ACTUAL_MS" -le "$THRESHOLD_MS" ]; then
        pass "$NAME (${ACTUAL_MS}ms <= ${THRESHOLD_MS}ms)"
    else
        fail "$NAME" "${ACTUAL_MS}ms exceeded ${THRESHOLD_MS}ms"
    fi
}

# bench_report: print a benchmark metric line (informational, no pass/fail).
bench() {
    printf "  \033[36mBENCH\033[0m %s: %s\n" "$1" "$2"
}

# run_skill_timed: runs a skill and returns the duration_ms from the response.
run_skill_timed() {
    BODY="$1"
    RESP=$(curl -s -X POST "$API_URL/v1/executions" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        -d "$BODY" 2>/dev/null)
    # Extract duration_ms from response JSON
    DUR=$(echo "$RESP" | grep -o '"duration_ms":[0-9]*' | head -1 | sed 's/"duration_ms"://')
    [ -z "$DUR" ] && DUR=0
    echo "$DUR"
}

# ###############################################
section "27. PERF: API endpoint latency"
# ###############################################

# Health endpoint should be sub-50ms (no DB, no auth)
T=$(curl_ms "$API_URL/health")
check_under_ms "GET /health latency" "$T" 50
bench "/health" "${T}ms"

# Ready endpoint (hits Postgres)
T=$(curl_ms "$API_URL/ready")
check_under_ms "GET /ready latency" "$T" 100
bench "/ready" "${T}ms"

# Auth + list skills
T=$(curl_ms -H "Authorization: Bearer $API_KEY" "$API_URL/v1/skills")
check_under_ms "GET /v1/skills latency" "$T" 200
bench "/v1/skills" "${T}ms"

# Get skill metadata (download + parse zip from MinIO)
T=$(curl_ms -H "Authorization: Bearer $API_KEY" "$API_URL/v1/skills/data-analysis/1.0.0")
check_under_ms "GET /v1/skills/:name/:version latency" "$T" 200
bench "/v1/skills/:n/:v" "${T}ms"

# Auth rejection should be fast (no backend work)
T=$(curl_ms -H "Authorization: Bearer wrong-key" "$API_URL/v1/skills")
check_under_ms "401 rejection latency" "$T" 50
bench "401 reject" "${T}ms"

# ###############################################
section "28. PERF: Single execution latency"
# ###############################################

# Minimal skill (echo) — measures pure overhead: API → registry → Docker create → copy → start → wait → copy out → respond
T=$(run_skill_timed '{"skill": "bash-echo", "version": "1.0.0", "input": {"x": 1}}')
check_under_ms "Echo skill execution" "$T" 15000
bench "echo skill" "${T}ms"

# Word-counter (slightly more work: tokenize + count + write artifact)
T=$(run_skill_timed '{"skill": "word-counter", "version": "1.0.0", "input": {"text": "benchmark test run"}}')
check_under_ms "Word-counter execution" "$T" 15000
bench "word-counter" "${T}ms"

# Env-check (reads env, writes JSON — minimal compute)
T=$(run_skill_timed '{"skill": "env-check", "version": "1.0.0", "input": {"check_vars": ["HOME"]}}')
check_under_ms "Env-check execution" "$T" 15000
bench "env-check" "${T}ms"

# ###############################################
section "29. PERF: Execution overhead (cold vs warm)"
# ###############################################

# "Cold" = first execution after upload (image + container setup).
# "Warm" = subsequent execution (image cached by Docker daemon).
# We've already run many executions above, so everything is "warm" now.
# Run the same skill 3 times and check that timings are stable.

T1=$(run_skill_timed '{"skill": "bash-echo", "version": "1.0.0", "input": {"i": 1}}')
T2=$(run_skill_timed '{"skill": "bash-echo", "version": "1.0.0", "input": {"i": 2}}')
T3=$(run_skill_timed '{"skill": "bash-echo", "version": "1.0.0", "input": {"i": 3}}')

bench "echo warm run 1" "${T1}ms"
bench "echo warm run 2" "${T2}ms"
bench "echo warm run 3" "${T3}ms"

# Compute average
AVG=$(( (T1 + T2 + T3) / 3 ))
bench "echo avg (3 runs)" "${AVG}ms"

# Ensure no single run is wildly slower than the average (> 3x = regression)
TESTS=$((TESTS + 1))
WORST=$T1
[ "$T2" -gt "$WORST" ] && WORST=$T2
[ "$T3" -gt "$WORST" ] && WORST=$T3
LIMIT=$(( AVG * 3 + 1000 ))
if [ "$WORST" -le "$LIMIT" ]; then
    pass "Warm execution consistency (worst ${WORST}ms <= 3x avg + 1s)"
else
    fail "Warm execution consistency" "worst=${WORST}ms, avg=${AVG}ms — too much variance"
fi

# ###############################################
section "30. PERF: Sequential throughput"
# ###############################################

# Run 5 executions sequentially, measure wall-clock time.
SEQ_COUNT=5
SEQ_START=$(date +%s)

SEQ_OK=0
for i in $(seq 1 $SEQ_COUNT); do
    RESULT=$(run_skill "{\"skill\": \"bash-echo\", \"version\": \"1.0.0\", \"input\": {\"seq\": $i}}")
    if echo "$RESULT" | grep -q '"status":"success"'; then
        SEQ_OK=$((SEQ_OK + 1))
    fi
done

SEQ_END=$(date +%s)
SEQ_ELAPSED=$((SEQ_END - SEQ_START))
# Avoid divide by zero
[ "$SEQ_ELAPSED" -eq 0 ] && SEQ_ELAPSED=1

SEQ_RPS=$(( SEQ_COUNT / SEQ_ELAPSED ))

check "$SEQ_COUNT sequential executions: all succeeded" "$SEQ_COUNT" "$SEQ_OK"

TESTS=$((TESTS + 1))
if [ "$SEQ_ELAPSED" -le 120 ]; then
    pass "Sequential throughput: ${SEQ_COUNT} in ${SEQ_ELAPSED}s (~${SEQ_RPS}/s)"
else
    fail "Sequential throughput" "${SEQ_ELAPSED}s for $SEQ_COUNT executions (too slow)"
fi

bench "sequential ${SEQ_COUNT}x echo" "${SEQ_ELAPSED}s total, ~${SEQ_RPS} exec/s"

# ###############################################
section "31. PERF: Concurrent throughput"
# ###############################################

# Fire 5 executions in parallel, measure wall-clock time.
CONC_COUNT=5
CONC_DIR="/tmp/perf_conc_$$"
mkdir -p "$CONC_DIR"

CONC_START=$(date +%s)

for i in $(seq 1 $CONC_COUNT); do
    curl -s -X POST "$API_URL/v1/executions" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        -d "{\"skill\": \"bash-echo\", \"version\": \"1.0.0\", \"input\": {\"conc\": $i}}" \
        > "$CONC_DIR/result_$i.json" 2>/dev/null &
done
wait

CONC_END=$(date +%s)
CONC_ELAPSED=$((CONC_END - CONC_START))
[ "$CONC_ELAPSED" -eq 0 ] && CONC_ELAPSED=1

CONC_OK=0
for i in $(seq 1 $CONC_COUNT); do
    if grep -q '"status":"success"' "$CONC_DIR/result_$i.json" 2>/dev/null; then
        CONC_OK=$((CONC_OK + 1))
    fi
done

CONC_RPS=$(( CONC_COUNT / CONC_ELAPSED ))

check "Concurrent ${CONC_COUNT}x: all succeeded" "$CONC_COUNT" "$CONC_OK"

TESTS=$((TESTS + 1))
if [ "$CONC_ELAPSED" -le 60 ]; then
    pass "Concurrent throughput: ${CONC_COUNT} in ${CONC_ELAPSED}s (~${CONC_RPS}/s)"
else
    fail "Concurrent throughput" "${CONC_ELAPSED}s for $CONC_COUNT executions (too slow)"
fi

bench "concurrent ${CONC_COUNT}x echo" "${CONC_ELAPSED}s wall-clock, ~${CONC_RPS} exec/s"

# Check that concurrent is faster than sequential
TESTS=$((TESTS + 1))
if [ "$CONC_ELAPSED" -le "$SEQ_ELAPSED" ]; then
    pass "Concurrent faster than sequential (${CONC_ELAPSED}s <= ${SEQ_ELAPSED}s)"
else
    # Not a hard failure — could be resource-constrained
    fail "Concurrent vs sequential" "concurrent=${CONC_ELAPSED}s, sequential=${SEQ_ELAPSED}s"
fi

rm -rf "$CONC_DIR"

# ###############################################
section "32. PERF: Concurrent scaling (1, 3, 5, 10)"
# ###############################################

run_concurrent_batch() {
    N="$1"
    BATCH_DIR="/tmp/perf_scale_${N}_$$"
    mkdir -p "$BATCH_DIR"

    BATCH_START=$(date +%s)

    for i in $(seq 1 "$N"); do
        curl -s -X POST "$API_URL/v1/executions" \
            -H "Authorization: Bearer $API_KEY" \
            -H "Content-Type: application/json" \
            -d "{\"skill\": \"bash-echo\", \"version\": \"1.0.0\", \"input\": {\"n\": $i}}" \
            > "$BATCH_DIR/r_$i.json" 2>/dev/null &
    done
    wait

    BATCH_END=$(date +%s)
    BATCH_ELAPSED=$((BATCH_END - BATCH_START))
    [ "$BATCH_ELAPSED" -eq 0 ] && BATCH_ELAPSED=1

    BATCH_OK=0
    for i in $(seq 1 "$N"); do
        if grep -q '"status":"success"' "$BATCH_DIR/r_$i.json" 2>/dev/null; then
            BATCH_OK=$((BATCH_OK + 1))
        fi
    done

    rm -rf "$BATCH_DIR"

    # Output: ok_count elapsed_seconds
    echo "$BATCH_OK $BATCH_ELAPSED"
}

PREV_ELAPSED=0
for BATCH_SIZE in 1 3 5 10; do
    READ_RESULT=$(run_concurrent_batch "$BATCH_SIZE")
    B_OK=$(echo "$READ_RESULT" | awk '{print $1}')
    B_ELAPSED=$(echo "$READ_RESULT" | awk '{print $2}')

    check "Scale ${BATCH_SIZE}x: all succeeded" "$BATCH_SIZE" "$B_OK"

    TESTS=$((TESTS + 1))
    if [ "$B_ELAPSED" -le 120 ]; then
        pass "Scale ${BATCH_SIZE}x: completed in ${B_ELAPSED}s"
    else
        fail "Scale ${BATCH_SIZE}x" "${B_ELAPSED}s (exceeded 120s budget)"
    fi

    B_RPS=$(( BATCH_SIZE / B_ELAPSED ))
    bench "scale ${BATCH_SIZE}x" "${B_ELAPSED}s, ~${B_RPS} exec/s"

    PREV_ELAPSED="$B_ELAPSED"
done

# ###############################################
section "33. PERF: Upload latency"
# ###############################################

# Measure how fast we can upload a skill zip
UPLOAD_ZIP="/tmp/perf_upload.zip"
(cd /skills/word-counter && zip -r "$UPLOAD_ZIP" . > /dev/null 2>&1)

# Upload timing — small skill zip (~2KB)
T=$(curl_ms -X POST "$API_URL/v1/skills" \
    -H "Authorization: Bearer $API_KEY" \
    -F "file=@$UPLOAD_ZIP" \
    -F "name=word-counter" \
    -F "version=1.0.0")
check_under_ms "Upload small skill (<2KB)" "$T" 500
bench "upload small skill" "${T}ms"

# Upload timing — larger zip with padding
mkdir -p /tmp/perf_big_skill/scripts
cp /skills/data-analysis/SKILL.md /tmp/perf_big_skill/SKILL.md
cp /skills/data-analysis/scripts/main.py /tmp/perf_big_skill/scripts/main.py
echo "# requirements" > /tmp/perf_big_skill/requirements.txt
# Create a ~100KB padding file
dd if=/dev/urandom bs=1024 count=100 2>/dev/null | base64 > /tmp/perf_big_skill/data_payload.txt
(cd /tmp/perf_big_skill && zip -r /tmp/perf_big_upload.zip . > /dev/null 2>&1)

T=$(curl_ms -X POST "$API_URL/v1/skills" \
    -H "Authorization: Bearer $API_KEY" \
    -F "file=@/tmp/perf_big_upload.zip" \
    -F "name=data-analysis" \
    -F "version=1.0.0")
check_under_ms "Upload medium skill (~140KB)" "$T" 2000
bench "upload medium skill" "${T}ms"

# ###############################################
section "34. PERF: Execution retrieval latency"
# ###############################################

# Run a skill and then measure how fast we can retrieve the result and logs
RESULT=$(run_skill '{"skill": "bash-echo", "version": "1.0.0", "input": {"perf": true}}')
PERF_EXEC_ID=$(echo "$RESULT" | grep -o '"execution_id":"[^"]*"' | head -1 | sed 's/"execution_id":"//;s/"//')

if [ -n "$PERF_EXEC_ID" ]; then
    T=$(curl_ms -H "Authorization: Bearer $API_KEY" "$API_URL/v1/executions/$PERF_EXEC_ID")
    check_under_ms "GET execution by ID" "$T" 100
    bench "GET /v1/executions/:id" "${T}ms"

    T=$(curl_ms -H "Authorization: Bearer $API_KEY" "$API_URL/v1/executions/$PERF_EXEC_ID/logs")
    check_under_ms "GET execution logs" "$T" 100
    bench "GET /v1/executions/:id/logs" "${T}ms"
else
    TESTS=$((TESTS + 2))
    fail "Perf exec ID extraction" "could not extract ID for retrieval benchmark"
fi

# ###############################################
section "35. PERF: Large payload execution"
# ###############################################

# Measure execution time with increasingly large inputs.
# 10 records
T10=$(run_skill_timed '{"skill": "data-analysis", "version": "1.0.0", "input": {"data": [{"a":1},{"a":2},{"a":3},{"a":4},{"a":5},{"a":6},{"a":7},{"a":8},{"a":9},{"a":10}]}}')
bench "data-analysis 10 rows" "${T10}ms"

# 50 records
INPUT_50='{"skill": "data-analysis", "version": "1.0.0", "input": {"data": ['
for i in $(seq 1 50); do
    [ "$i" -gt 1 ] && INPUT_50="${INPUT_50},"
    INPUT_50="${INPUT_50}{\"id\":$i,\"val\":$((i*3))}"
done
INPUT_50="${INPUT_50}]}}"
T50=$(run_skill_timed "$INPUT_50")
bench "data-analysis 50 rows" "${T50}ms"

# 200 records
INPUT_200='{"skill": "data-analysis", "version": "1.0.0", "input": {"data": ['
for i in $(seq 1 200); do
    [ "$i" -gt 1 ] && INPUT_200="${INPUT_200},"
    INPUT_200="${INPUT_200}{\"id\":$i,\"val\":$((i*3)),\"cat\":\"group_$((i%5))\"}"
done
INPUT_200="${INPUT_200}]}}"
T200=$(run_skill_timed "$INPUT_200")
bench "data-analysis 200 rows" "${T200}ms"

check_under_ms "200-row execution" "$T200" 30000
bench "10→200 row scaling" "${T10}ms → ${T50}ms → ${T200}ms"

# Check that scaling is sub-linear (200 rows shouldn't be 20x slower than 10 rows)
TESTS=$((TESTS + 1))
if [ "$T10" -gt 0 ]; then
    RATIO=$(( T200 / T10 ))
    if [ "$RATIO" -le 10 ]; then
        pass "Sub-linear scaling: 200 rows only ${RATIO}x slower than 10 rows"
    else
        fail "Scaling" "200 rows is ${RATIO}x slower than 10 (expected <= 10x)"
    fi
else
    pass "Sub-linear scaling: 10-row baseline too fast to measure ratio"
fi

# ###############################################
section "36. PERF: Multi-file artifact overhead"
# ###############################################

# Measure how artifact count affects execution time.
# 1 file
T1F=$(run_skill_timed '{"skill": "multi-file-output", "version": "1.0.0", "input": {"file_count": 1}}')
bench "1 artifact file" "${T1F}ms"

# 10 files
T10F=$(run_skill_timed '{"skill": "multi-file-output", "version": "1.0.0", "input": {"file_count": 10}}')
bench "10 artifact files" "${T10F}ms"

# 25 files
T25F=$(run_skill_timed '{"skill": "multi-file-output", "version": "1.0.0", "input": {"file_count": 25}}')
bench "25 artifact files" "${T25F}ms"

check_under_ms "25-file artifact execution" "$T25F" 30000
bench "artifact scaling" "1→${T1F}ms, 10→${T10F}ms, 25→${T25F}ms"

# ###############################################
section "37. PERF: Rapid-fire API requests"
# ###############################################

# Measure how the API handles a burst of lightweight requests (no execution).
# 20 sequential health checks.
BURST_START=$(date +%s)
BURST_OK=0
for i in $(seq 1 20); do
    CODE=$(http_code "$API_URL/health")
    [ "$CODE" = "200" ] && BURST_OK=$((BURST_OK + 1))
done
BURST_END=$(date +%s)
BURST_ELAPSED=$((BURST_END - BURST_START))
[ "$BURST_ELAPSED" -eq 0 ] && BURST_ELAPSED=1

check "Rapid-fire 20x /health: all 200" "20" "$BURST_OK"
check_under_ms "Rapid-fire 20x /health total" "$((BURST_ELAPSED * 1000))" 5000
bench "20x /health burst" "${BURST_ELAPSED}s total"

# 20 sequential authenticated list-skills requests
BURST_START=$(date +%s)
BURST_OK=0
for i in $(seq 1 20); do
    CODE=$(http_code -H "Authorization: Bearer $API_KEY" "$API_URL/v1/skills")
    [ "$CODE" = "200" ] && BURST_OK=$((BURST_OK + 1))
done
BURST_END=$(date +%s)
BURST_ELAPSED=$((BURST_END - BURST_START))
[ "$BURST_ELAPSED" -eq 0 ] && BURST_ELAPSED=1

check "Rapid-fire 20x /v1/skills: all 200" "20" "$BURST_OK"
check_under_ms "Rapid-fire 20x /v1/skills total" "$((BURST_ELAPSED * 1000))" 10000
bench "20x /v1/skills burst" "${BURST_ELAPSED}s total"

# ###############################################
section "Results"
# ###############################################
echo ""
PASSED=$((TESTS - FAILURES))
printf "Tests: %d total, \033[32m%d passed\033[0m, \033[31m%d failed\033[0m\n" "$TESTS" "$PASSED" "$FAILURES"
echo ""

if [ "$FAILURES" -gt 0 ]; then
    printf "\033[31mSOME TESTS FAILED\033[0m\n"
    exit 1
else
    printf "\033[32mALL TESTS PASSED\033[0m\n"
    exit 0
fi
