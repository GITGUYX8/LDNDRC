#!/bin/bash
# Unit-style tests for the hardware detection logic in joining-cluster.sh
# (the onboarding-portal enrollment agent served by the control panel).
#
# The canonical script lives at
# control-panel/internal/portal/assets/joining-cluster.sh (go:embed) —
# this runner exercises it via --detect-only, which prints the detection
# and decision lines without touching the network, the panel, or root.
#
# Runs without a cluster, Docker, VMs, or root — ~2 seconds.
set -euo pipefail

cd "$(dirname "$0")"
SCRIPT="control-panel/internal/portal/assets/joining-cluster.sh"
PASS=0
FAIL=0

# test_scenario LABEL EXPECTED [TEST_CPU=.. TEST_RAM=.. TEST_GPU=..]
test_scenario() {
    local label="$1"
    local expected="$2"
    shift 2

    echo -n "[test] ${label} ... "
    output=$(env "$@" bash "$SCRIPT" --detect-only 2>&1 || true)

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

echo "=== joining-cluster.sh Hardware Detection Tests ==="
echo ""

# Scenario 1: Meets all Host requirements → HOST profile
test_scenario \
    "Host — meets all requirements" \
    "Profile: HOST" \
    TEST_CPU=12 TEST_RAM=16 TEST_GPU=nvidia

# Scenario 2: Falls below all thresholds → Guest
test_scenario \
    "Guest — below all thresholds" \
    "Profile: GUEST" \
    TEST_CPU=4 TEST_RAM=8 TEST_GPU=none

# Scenario 3: CPU+RAM meet, no GPU → Guest (AND-logic)
test_scenario \
    "Guest — CPU+RAM meet, no GPU" \
    "Profile: GUEST" \
    TEST_CPU=12 TEST_RAM=16 TEST_GPU=none

# Scenario 4: GPU meets, CPU below → Guest (AND-logic, other direction)
test_scenario \
    "Guest — GPU meets, CPU below threshold" \
    "Profile: GUEST" \
    TEST_CPU=4 TEST_RAM=16 TEST_GPU=nvidia

# Scenario 5: RAM one GB below the gate → Guest (boundary check)
test_scenario \
    "Guest — RAM one GB short" \
    "Profile: GUEST" \
    TEST_CPU=12 TEST_RAM=15 TEST_GPU=nvidia

echo ""
echo "=== Results: ${PASS} passed, ${FAIL} failed ==="
[ "$FAIL" -eq 0 ] || exit 1
