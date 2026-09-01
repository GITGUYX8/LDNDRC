#!/bin/bash
# Unit-style tests for the hardware detection logic in join-cluster.sh.
#
# Every scenario feeds a different TEST_* hardware profile and asserts the
# expected decision string. The Host branch would normally curl the k3s
# installer, so MASTER_IP/NODE_TOKEN are stubbed and failures are tolerated
# (`|| true`): only the detection decision is under test here.
#
# Runs without a cluster, Docker, VMs, or root — 4 scenarios in ~2 seconds.
set -euo pipefail

cd "$(dirname "$0")"
PASS=0
FAIL=0

# test_scenario LABEL EXPECTED TEST_CPU=.. TEST_RAM=.. TEST_GPU=..
# Runs join-cluster.sh with the given overrides and greps its output for
# EXPECTED, which selects between the Host and Guest branches.
test_scenario() {
    local label="$1"
    local expected="$2"
    shift 2

    echo -n "[test] ${label} ... "
    # env passes the remaining VAR=value pairs as a clean environment
    # instead of relying on shell assignment syntax inside a subshell.
    output=$(env MASTER_IP=127.0.0.1 NODE_TOKEN=test "$@" bash join-cluster.sh 2>&1 || true)

    if echo "$output" | grep -q "$expected"; then
        echo "PASS"
        PASS=$((PASS + 1))
    else
        echo "FAIL"
        echo "       Expected output to contain: ${expected}"
        echo "       Got: ${output}" | head -5
        FAIL=$((FAIL + 1))
    fi
}

echo "=== join-cluster.sh Hardware Detection Tests ==="
echo ""

# Scenario 1: Meets all Host requirements → joins cluster
test_scenario \
    "Host — meets all requirements" \
    "Joining as HOST" \
    TEST_CPU=12 TEST_RAM=16 TEST_GPU=nvidia

# Scenario 2: Falls below all thresholds → guest
test_scenario \
    "Guest — below all thresholds" \
    "Use browser" \
    TEST_CPU=4 TEST_RAM=4 TEST_GPU=none

# Scenario 3: CPU+RAM meet, no GPU → guest
# The AND-logic means one failing requirement forces Guest mode.
test_scenario \
    "Guest — CPU+RAM meet, no GPU" \
    "Use browser" \
    TEST_CPU=12 TEST_RAM=16 TEST_GPU=none

# Scenario 4: Only GPU meets, CPU below → guest
# Same AND-logic check from the other direction.
test_scenario \
    "Guest — GPU meets, CPU below threshold" \
    "Use browser" \
    TEST_CPU=4 TEST_RAM=16 TEST_GPU=nvidia

echo ""
echo "=== Results: ${PASS} passed, ${FAIL} failed ==="
[ "$FAIL" -eq 0 ] || exit 1