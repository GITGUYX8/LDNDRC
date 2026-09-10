#!/bin/bash
# Second-laptop acceptance for ldndrc-join (run ON the second laptop).
#
# Automates everything scriptable: reachability, binary download +
# checksum verification, and the --plain hardware check. The interactive
# TUI matrix stays manual (it needs a human at a terminal), and the
# register rehearsal stays paired (approval happens live on the master).
#
#   MASTER_IP=192.168.1.37 ./acceptance-second-laptop.sh
#
# Env knobs: MASTER_IP (default below), VERSION (default v0.2.0),
# FLAVOR (default linux-amd64; linux-arm64, darwin-amd64, darwin-arm64,
# windows-amd64.exe also published).
set -euo pipefail

MASTER_IP="${MASTER_IP:-192.168.1.37}"
VERSION="${VERSION:-v0.2.0}"
FLAVOR="${FLAVOR:-linux-amd64}"
ASSET="ldndrc-join-${FLAVOR}"
BASE="https://github.com/GITGUYX8/LDNDRC/releases/download/${VERSION}"

fail() {
    echo "[acceptance] FAIL: $1" >&2
    exit 1
}

echo "=== 0. Reachability (master ${MASTER_IP}) ==="
ping -c 3 "${MASTER_IP}" > /dev/null \
    || fail "cannot ping ${MASTER_IP} — same Wi-Fi? See network debug notes."
HEALTH="$(curl -s -m 10 -H "Host: control.ros-platform.local" "http://${MASTER_IP}:80/healthz")"
[ "${HEALTH}" = '{"status":"ok"}' ] \
    || fail "API health check failed (got: ${HEALTH}) — is Traefik up on the master?"
echo "[acceptance] master reachable, API healthy"

echo ""
echo "=== 1. Download + verify ${ASSET} ${VERSION} ==="
curl -sSL "${BASE}/${ASSET}" -o "${ASSET}" \
    || fail "download failed — check the release exists."
curl -sSL "${BASE}/SHA256SUMS" -o SHA256SUMS \
    || fail "checksum file download failed."
sha256sum -c SHA256SUMS --ignore-missing \
    || fail "checksum mismatch — do NOT run the binary."
chmod +x "${ASSET}"
echo "[acceptance] checksum OK"

echo ""
echo "=== 2. Hardware check (--plain, zero system changes) ==="
"./${ASSET}" --plain
echo "[acceptance] exit: $? (0 either way; Guest on below-bar hardware is a PASS)"

echo ""
echo "=== 3. Interactive matrix (manual) ==="
echo "Run: ./${ASSET}"
echo "Describe or screenshot the check rows, then quit with 'q'."
echo "Do NOT press 'j' yet — the register rehearsal is paired live."
echo ""
echo "=== 4. Register rehearsal (paired, still zero installs) ==="
echo "TEST_CPU=12 TEST_RAM=32 TEST_GPU=nvidia MASTER_IP=${MASTER_IP} ./${ASSET} --plain"
echo "Then tell the master operator to approve, and poll for the one-time token."
