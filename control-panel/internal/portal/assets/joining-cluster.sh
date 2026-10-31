#!/usr/bin/env bash
# joining-cluster.sh — LDNDRC enrollment agent.
#
# Downloaded from the master laptop's onboarding portal
# (http://<master-ip>:8082/portal) while on the onboarding Wi-Fi.
#
# What it does:
#   1. Profiles this laptop — same hardware gates as join-cluster.sh:
#      >= 8 cores, >= 16 GB RAM, NVIDIA GPU => Host, otherwise Guest.
#   2. Finds the control panel: PANEL_URL env, then mDNS browse
#      (_ldndrc-master._tcp), then an interactive prompt.
#   3. Registers the device with the panel (POST /api/nodes/register).
#      Re-running is safe — the server deduplicates registrations by
#      hardware fingerprint, so you always get the same join request.
#   4. Polls GET /api/nodes/{id} until the instructor approves or
#      denies the request (status comes only from the server; this
#      script has no way to approve itself).
#   5. Approved Hosts are offered the K3s agent install (needs sudo +
#      internet for get.k3s.io — override K3S_INSTALL_URL for a local
#      mirror). All approved laptops are switched to the cluster
#      Wi-Fi when possible (nmcli), else given manual instructions.
#
# Network access itself is controlled by the gateway/cluster router —
# approval in the panel is the paperwork, not the firewall.
#
# Usage:
#   joining-cluster.sh              full enrollment flow
#   joining-cluster.sh --detect-only  print the detection lines and exit
#                                     (CI regression check, no network)
#
# Environment:
#   PANEL_URL       e.g. http://192.168.4.1:8082 (skips mDNS/prompt)
#   JOIN_KEY        shared enrollment key, if the panel requires one
#   CLUSTER_SSID    cluster Wi-Fi network to switch to after approval
#   ENROLL_TIMEOUT  seconds to wait for approval (default 600)
#   K3S_INSTALL_URL k3s installer (default https://get.k3s.io)
# Testing on one machine (see test-join.sh): TEST_CPU TEST_RAM TEST_GPU
# TEST_HOSTNAME (different fingerprint → separate join request)
set -euo pipefail

TAG="[joining-cluster]"

log() { printf '%s %s\n' "$TAG" "$*"; }
die() { log "ERROR: $*"; exit 1; }

# --- JSON helpers (jq when present, flat-JSON fallback otherwise) -------
json_str() {
    # json_str <json> <key>: value of a flat JSON field; empty when absent.
    local json=$1 key=$2 v="" line
    if command -v jq >/dev/null 2>&1; then
        v=$(printf '%s' "$json" | jq -r --arg k "$key" '.[$k] // empty' 2>/dev/null || true)
        if [ -n "$v" ]; then
            printf '%s\n' "$v"
            return 0
        fi
        # jq parsed the document fine and the key is absent/empty — do
        # not let a malformed response fall through to the weak parser.
        if printf '%s' "$json" | jq -e . >/dev/null 2>&1; then
            return 0
        fi
    fi
    line=$(printf '%s' "$json" | tr ',{}' '\n\n ' | tr -d '"' | grep -E "^${key}:" | head -n 1 || true)
    [ -n "$line" ] && printf '%s\n' "$line" | cut -d: -f2-
    return 0
}

# --- Hardware detection (mirrors join-cluster.sh, TEST_* overridable) ---
CPU=0 RAM_GB=0 HAS_GPU=0 IS_HOST=0
OS=$(uname -s 2>/dev/null || echo linux)
ARCH=$(uname -m 2>/dev/null || echo unknown)

if [ -n "${TEST_CPU:-}" ]; then CPU=$TEST_CPU
elif command -v nproc >/dev/null 2>&1; then CPU=$(nproc)
elif command -v sysctl >/dev/null 2>&1; then CPU=$(sysctl -n hw.ncpu 2>/dev/null || echo 0)
fi

if [ -n "${TEST_RAM:-}" ]; then RAM_GB=$TEST_RAM
elif [ -r /proc/meminfo ]; then
    RAM_GB=$(( $(grep MemTotal /proc/meminfo | awk '{print $2}') / 1024 / 1024 ))
elif command -v sysctl >/dev/null 2>&1; then
    RAM_GB=$(( $(sysctl -n hw.memsize 2>/dev/null || echo 0) / 1024 / 1024 / 1024 ))
fi

if [ -n "${TEST_GPU:-}" ]; then
    if [ "$TEST_GPU" = "none" ]; then HAS_GPU=0; else HAS_GPU=1; fi
elif command -v lspci >/dev/null 2>&1; then
    HAS_GPU=$(lspci 2>/dev/null | grep -c -i -E 'vga|3d|nvidia' || true)
    [ -n "$HAS_GPU" ] || HAS_GPU=0
fi

if [ "$CPU" -ge 8 ] && [ "$RAM_GB" -ge 16 ] && [ "$HAS_GPU" -ge 1 ]; then
    IS_HOST=1
fi

detected_line() {
    log "Detected: CPU=${CPU} cores, RAM=${RAM_GB}GB, GPU=$([ "$HAS_GPU" -ge 1 ] && echo yes || echo no)"
}
decision_line() {
    if [ "$IS_HOST" -eq 1 ]; then
        log "High-end laptop detected. Profile: HOST (compute contributor)."
    else
        log "Profile: GUEST (browser-only thin client). Registering anyway — the instructor sees every laptop."
    fi
}

# --- Find the control panel ---------------------------------------------
discover_master() {
    if [ -n "${PANEL_URL:-}" ]; then
        printf '%s\n' "${PANEL_URL%/}"
        return 0
    fi
    if command -v avahi-browse >/dev/null 2>&1; then
        local out ip port
        out=$(avahi-browse -r -t _ldndrc-master._tcp 2>/dev/null || true)
        ip=$(printf '%s\n' "$out" | sed -n 's/.*address = \[\([0-9.]*\)\].*/\1/p' | head -n 1)
        port=$(printf '%s\n' "$out" | sed -n 's/.*port = \[\([0-9]*\)\].*/\1/p' | head -n 1)
        if [ -n "$ip" ]; then
            printf 'http://%s:%s\n' "$ip" "${port:-8082}"
            return 0
        fi
    fi
    return 1
}

# --- Register (retries; server-side idempotency makes re-runs safe) -----
# Sets REG_BODY (global) to the registration JSON; diagnostics go to
# stdout — never capture log() output with command substitution.
register() {
    local attempt resp code body hn
    hn="${TEST_HOSTNAME:-$(hostname 2>/dev/null || echo laptop)}"  # RFC 952 hostnames are JSON-safe; keep it that way
    hn=$(printf '%s' "$hn" | tr -c 'A-Za-z0-9._-' '_')
    local payload
    payload=$(printf '{"hostname":"%s","os":"%s","arch":"%s","cpu":%d,"ram_gb":%d,"gpu":"%s","join_key":"%s"}' \
        "$hn" "$OS" "$ARCH" "$CPU" "$RAM_GB" \
        "$([ "$HAS_GPU" -ge 1 ] && echo nvidia || echo none)" "${JOIN_KEY:-}")

    for attempt in 1 2 3; do
        resp=$(curl -sS -m 10 -w '\n%{http_code}' -X POST \
            -H 'Content-Type: application/json' -d "$payload" \
            "$PANEL_URL/api/nodes/register" 2>&1) || {
            log "register attempt $attempt failed: $(printf '%s' "$resp" | head -n 1)"
            sleep "$((attempt * 2))"
            continue
        }
        code=$(printf '%s\n' "$resp" | tail -n 1)
        body=$(printf '%s\n' "$resp" | sed '$d')
        case "$code" in
            200 | 201)
                REG_BODY=$body
                return 0
                ;;
            403) die "join key rejected — ask the instructor for the JOIN_KEY value" ;;
            429) log "rate limited, waiting 10s"; sleep 10 ;;
            *) log "register attempt $attempt: HTTP $code"; sleep "$((attempt * 2))" ;;
        esac
    done
    log "ERROR: could not register after 3 attempts — check the panel URL ($PANEL_URL)"
    return 1
}

# --- Wait for the instructor's decision ---------------------------------
# Sets POLL_BODY (global) on success. Polls parse every response body —
# an approved node answers 503 while the join/token service is not
# wired (e.g. K3S_JOIN_URL unset), and dropping `-f` errors would mean
# never noticing the approval at all.
wait_approval() {
    local deadline=$((SECONDS + ENROLL_TIMEOUT)) code status resp
    while [ "$SECONDS" -lt "$deadline" ]; do
        if ! resp=$(curl -sS -m 10 -w '\n%{http_code}' "$PANEL_URL/api/nodes/$NODE_ID" 2>/dev/null); then
            sleep 5
            continue
        fi
        code=$(printf '%s\n' "$resp" | tail -n 1)
        POLL_BODY=$(printf '%s\n' "$resp" | sed '$d')
        case "$code" in
            200)
                status=$(json_str "$POLL_BODY" status)
                case "$status" in
                    pending)
                        log "Waiting for instructor approval… (Ctrl-C keeps the request pending)"
                        sleep 5
                        ;;
                    approved | joined)
                        return 0
                        ;;
                    denied)
                        log "DENIED by the instructor: $(json_str "$POLL_BODY" reason)"
                        return 1
                        ;;
                    expired)
                        log "Request expired — run this script again to re-request access."
                        return 1
                        ;;
                    *) sleep 5 ;;
                esac
                ;;
            404)
                log "Request no longer exists — run this script again to re-register."
                return 1
                ;;
            *)
                # 429 rate limit, 5xx transient panel trouble: keep polling.
                # Exception: an approved node answers 503 with one of these
                # bodies while the join/token service is not wired up
                # (minter missing or K3S_JOIN_URL unset). The approval
                # itself is real — proceed with the network switch and
                # tell the student what is missing.
                case "$(json_str "$POLL_BODY" error)" in
                    "join service unavailable" | "join URL not configured")
                        log "APPROVED — but the panel cannot mint a join token yet (join service not configured). The instructor should check K3S_JOIN_URL."
                        return 0
                        ;;
                    *) sleep 5 ;;
                esac
                ;;
        esac
    done
    log "Timed out after ${ENROLL_TIMEOUT}s — your request stays pending; re-run this script to check again."
    return 1
}

# --- Approved Hosts: install the K3s agent ------------------------------
join_k3s() {
    # The approving poll carries k3s_url + k3s_token once the panel
    # mints them (server-side; the script never fabricates them).
    local k3s_url k3s_token answer
    k3s_url=$(json_str "$POLL_BODY" k3s_url)
    k3s_token=$(json_str "$POLL_BODY" k3s_token)
    if [ -z "$k3s_url" ] || [ -z "$k3s_token" ]; then
        log "Join token not minted yet — re-run this script in a minute to join."
        return 0
    fi
    printf '%s Join the K3s cluster as a compute Host now? [y/N] ' "$TAG" >&2
    if [ -t 0 ] && read -r answer && printf '%s' "$answer" | grep -qi '^y'; then
        log "Installing K3s agent from ${K3S_INSTALL_URL:-https://get.k3s.io} (needs sudo)…"
        if [ "$(id -u)" -eq 0 ]; then
            curl -fsSL "${K3S_INSTALL_URL:-https://get.k3s.io}" | K3S_URL="$k3s_url" K3S_TOKEN="$k3s_token" sh -s - agent
        else
            curl -fsSL "${K3S_INSTALL_URL:-https://get.k3s.io}" | sudo -E K3S_URL="$k3s_url" K3S_TOKEN="$k3s_token" sh -s - agent
        fi
        log "K3s agent installed — the instructor console will show this laptop as joined."
    else
        log "Skipped cluster join. Re-run this script later, or use the ldndrc-join tool."
    fi
}

# --- Switch to the cluster network --------------------------------------
switch_wifi() {
    local ssid="${CLUSTER_SSID:-}"
    if [ -z "$ssid" ] && [ -t 0 ]; then
        printf '%s Cluster Wi-Fi name (Enter to skip and switch manually): ' "$TAG" >&2
        read -r ssid
    fi
    if [ -z "$ssid" ]; then
        log "Manual step: switch Wi-Fi to the cluster network (ask the instructor for its name/password)."
        return 0
    fi
    if command -v nmcli >/dev/null 2>&1; then
        if nmcli --ask device wifi connect "$ssid"; then
            log "Connected to '$ssid'."
            return 0
        fi
        log "Could not switch automatically — connect to '$ssid' manually."
        return 1
    fi
    log "No nmcli found — connect to '$ssid' manually (e.g. desktop network menu)."
    return 0
}

main() {
    command -v curl >/dev/null 2>&1 || die "curl is required (sudo apt install curl)"
    detected_line
    decision_line

    if ! PANEL_URL=$(discover_master); then
        printf '%s Master panel URL (ask the instructor, e.g. http://192.168.4.1:8082): ' "$TAG" >&2
        read -r PANEL_URL
        [ -n "$PANEL_URL" ] || die "no panel URL given"
        PANEL_URL="${PANEL_URL%/}"
    fi
    log "Control panel: $PANEL_URL"

    if ! register; then
        exit 1
    fi
    NODE_ID=$(json_str "$REG_BODY" id)
    [ -n "$NODE_ID" ] || { log "ERROR: registration response had no node id: $REG_BODY"; exit 1; }
    log "Registered as node $NODE_ID (status: $(json_str "$REG_BODY" status))."

    if ! wait_approval; then
        exit 1
    fi
    log "APPROVED!"

    if [ "$IS_HOST" -eq 1 ]; then
        join_k3s
    fi
    switch_wifi || true

    log "Done. Once on the cluster network, open http://control.ros-platform.local"
}

case "${1:-}" in
    --detect-only)
        detected_line
        decision_line
        exit 0
        ;;
    "") main ;;
    *) die "usage: joining-cluster.sh [--detect-only]" ;;
esac
