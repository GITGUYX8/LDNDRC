#!/bin/bash
# Regression tests for tools/captive-portal/captive-trap.py — the server
# that answers intercepted port-80 traffic with a 302 to the portal.
#
# Runs entirely unprivileged: no nftables, no dnsmasq, no Wi-Fi change,
# no panel build. It starts the trap on a free localhost port with a
# fixed --panel-url and checks the redirect contract OS captive-portal
# probes depend on.
#
#   ./test-captive-trap.sh        # ~2 seconds
set -euo pipefail

cd "$(dirname "$0")"
TRAP="tools/captive-portal/captive-trap.py"
PORTAL="http://10.9.8.7:8082/portal"
PASS=0
FAIL=0
WORK="$(mktemp -d)"
PORT="$(python3 -c 'import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()')"
BASE="http://127.0.0.1:$PORT"
HDR="$WORK/headers"
BODY="$WORK/body"

cleanup() {
    [ -n "${TRAP_PID:-}" ] && kill "$TRAP_PID" 2>/dev/null || true
    rm -rf "$WORK"
}
trap cleanup EXIT

pass() { echo "PASS"; PASS=$((PASS + 1)); }
fail() {
    echo "FAIL"
    echo "       $1"
    FAIL=$((FAIL + 1))
}

expect_eq() { # label actual expected
    echo -n "[test] $1 ... "
    if [ "$2" = "$3" ]; then pass; else fail "expected '$3', got '$2'"; fi
}

expect_contains() { # label haystack needle
    echo -n "[test] $1 ... "
    if printf '%s' "$2" | grep -qF "$3"; then pass; else fail "expected to contain '$3'"; fi
}

header() { # name -> last value
    awk -v want="$1" 'BEGIN{IGNORECASE=1}
        $1 == want":" { $1=""; sub(/^ /, ""); val=$0 }
        END { gsub(/\r/, "", val); print val }' "$HDR"
}

echo "=== captive-trap.py redirect contract tests ==="
echo ""

python3 "$TRAP" --panel-url "$PORTAL" --listen-port "$PORT" \
    --bind 127.0.0.1 > "$WORK/trap.log" 2>&1 &
TRAP_PID=$!

for _ in $(seq 1 50); do
    curl -s -m 1 -o /dev/null "$BASE/" 2>/dev/null && break
    sleep 0.1
done
kill -0 "$TRAP_PID" 2>/dev/null || { cat "$WORK/trap.log"; exit 1; }

# 1-3: the happy path an OS probe hits (generate_204, hotspot-detect.html, …)
code="$(curl -s -o "$BODY" -D "$HDR" -w '%{http_code}' "$BASE/generate_204")"
expect_eq "GET probe answers 302" "$code" "302"
expect_eq "Location points at the portal" "$(header Location)" "$PORTAL"
expect_eq "Cache-Control stops probe caching" "$(header Cache-Control)" "no-store"

# 4-5: the body is a usable fallback for browsers that show it
expect_contains "body has a meta refresh" "$(cat "$BODY")" "http-equiv='refresh'"
expect_contains "body links the portal" "$(cat "$BODY")" "$PORTAL"

# 6-7: HEAD must not write a body (Content-Length 0), and POST (some
# captive apps post through the portal) must redirect too.
code="$(curl -s -I -o /dev/null -D "$HDR" -w '%{http_code}' "$BASE/captive")"
expect_eq "HEAD answers 302" "$code" "302"
expect_eq "HEAD sends no body" "$(header Content-Length)" "0"

code="$(curl -s -X POST -o /dev/null -D "$HDR" -w '%{http_code}' \
    -d 'probe=1' "$BASE/")"
expect_eq "POST answers 302" "$code" "302"
expect_eq "POST Location points at the portal" "$(header Location)" "$PORTAL"

# 8: --once is the config check run.sh relies on for a wired-in iface.
echo -n "[test] --once echoes the resolved portal URL ... "
once="$(python3 "$TRAP" --iface lo --panel-port 8082 --once 2>/dev/null || true)"
if printf '%s' "$once" | grep -qE '^http://[0-9.]+:8082/portal$'; then
    pass
else
    fail "got '$once'"
fi

echo ""
echo "=== Results: ${PASS} passed, ${FAIL} failed ==="
[ "$FAIL" -eq 0 ] || exit 1
