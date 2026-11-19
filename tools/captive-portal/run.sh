#!/usr/bin/env bash
# LDNDRC captive-portal rig — onboarding AP + interception + portal.
#
#   ./run.sh start    build/start panel + trap, render configs, print the
#                     privileged setup block (hotspot, dnsmasq, nft)
#   ./run.sh stop     stop the unprivileged processes; print the root teardown
#   ./run.sh status   what is up, what is intercepting, what is reachable
#   ./run.sh config   print the rendered config paths and portal URL
#   ./run.sh sudo     print only the privileged setup block
#
# The privileged parts (hotspot AP, dnsmasq, nftables) are printed, never
# run — they need root, and you should see exactly what will change.
#
# Everything the rig writes lives in $RUNTIME_DIR (default
# ~/.local/state/ldndrc-captive): the panel binary, JSON stores, logs and
# rendered configs. Nothing is committed and nothing lands in /etc.
set -euo pipefail

TOOL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$TOOL_DIR/../.." && pwd)"

RUNTIME_DIR="${RUNTIME_DIR:-$HOME/.local/state/ldndrc-captive}"
AP_IFACE="${AP_IFACE:-}"
AP_IP="${AP_IP:-192.168.4.1}"
DHCP_START="${DHCP_START:-192.168.4.50}"
DHCP_END="${DHCP_END:-192.168.4.150}"
PANEL_PORT="${PANEL_PORT:-8082}"
TRAP_PORT="${TRAP_PORT:-8090}"
SSID="${SSID:-ldndrc-onboarding}"
WIFI_PASSWORD="${WIFI_PASSWORD:-ldndrc-join-2026}"
CON_NAME="${CON_NAME:-ldndrc-onboarding}"
SKIP_BUILD="${SKIP_BUILD:-0}"

PANEL_BIN="$RUNTIME_DIR/panel"
PANEL_PID="$RUNTIME_DIR/panel.pid"
TRAP_PID="$RUNTIME_DIR/trap.pid"
TRAP_LOG="$RUNTIME_DIR/trap.log"
PANEL_LOG="$RUNTIME_DIR/panel.log"
NODES_DB="$RUNTIME_DIR/nodes.json"
DOWNLOADS_DB="$RUNTIME_DIR/downloads.json"
SESSIONS_DB="$RUNTIME_DIR/sessions.json"
DNSMASQ_CONF="$RUNTIME_DIR/dnsmasq-onboarding.conf"
DNSMASQ_PID="$RUNTIME_DIR/dnsmasq.pid"
DNSMASQ_LOG="$RUNTIME_DIR/dnsmasq.log"
NFT_FILE="$RUNTIME_DIR/ldndrc-captive.nft"

log()  { printf '[captive] %s\n' "$*"; }
warn() { printf '[captive] WARNING: %s\n' "$*" >&2; }
die()  { printf '[captive] ERROR: %s\n' "$*" >&2; exit 1; }

detect_iface() {
    local iface
    iface="$(nmcli -t -f DEVICE,TYPE device status 2>/dev/null \
        | awk -F: '$2=="wifi" {print $1; exit}')" || true
    [ -n "$iface" ] || iface="$(iw dev 2>/dev/null \
        | awk '$1=="Interface" {print $2; exit}')" || true
    [ -n "$iface" ] || iface="$(ip -o link show 2>/dev/null \
        | awk -F': ' '$2 ~ /^wl/ {print $2; exit}')" || true
    printf '%s' "$iface"
}

resolve_iface() {
    if [ -z "$AP_IFACE" ]; then
        AP_IFACE="$(detect_iface)"
        [ -n "$AP_IFACE" ] \
            || die "no Wi-Fi interface detected — set AP_IFACE=wlan0 (etc.)"
    fi
}

port_busy() {
    ss -ltn 2>/dev/null | awk '{print $4}' | grep -qE "[:.]$1$"
}

wait_http() { # url seconds
    local url="$1" deadline=$((SECONDS + ${2:-10}))
    while [ "$SECONDS" -lt "$deadline" ]; do
        curl -fsS -m 1 -o /dev/null "$url" 2>/dev/null && return 0
        sleep 0.2
    done
    return 1
}

render_configs() {
    resolve_iface
    mkdir -p "$RUNTIME_DIR"
    [ -f "$DNSMASQ_LOG" ] || : > "$DNSMASQ_LOG"

    sed -e "s|@IFACE@|$AP_IFACE|g" \
        -e "s|@TRAP_PORT@|$TRAP_PORT|g" \
        "$TOOL_DIR/ldndrc-captive.nft.tmpl" > "$NFT_FILE"

    sed -e "s|@IFACE@|$AP_IFACE|g" \
        -e "s|@AP_IP@|$AP_IP|g" \
        -e "s|@DHCP_START@|$DHCP_START|g" \
        -e "s|@DHCP_END@|$DHCP_END|g" \
        -e "s|@PIDFILE@|$DNSMASQ_PID|g" \
        -e "s|@LOGFILE@|$DNSMASQ_LOG|g" \
        "$TOOL_DIR/dnsmasq-onboarding.conf.tmpl" > "$DNSMASQ_CONF"

    log "rendered $NFT_FILE"
    log "rendered $DNSMASQ_CONF"
}

build_panel() {
    [ "$SKIP_BUILD" = "1" ] && { log "SKIP_BUILD=1 — reusing $PANEL_BIN"; return 0; }
    log "building control panel -> $PANEL_BIN"
    ( cd "$ROOT/control-panel" && go build -o "$PANEL_BIN" ./cmd/server )
}

start_panel() {
    if [ -f "$PANEL_PID" ] && kill -0 "$(cat "$PANEL_PID")" 2>/dev/null; then
        log "panel already running (pid $(cat "$PANEL_PID"))"
        return 0
    fi
    port_busy "$PANEL_PORT" \
        && die "port $PANEL_PORT is already in use (a deployed panel pod owns it?) — set PANEL_PORT=8083"

    log "starting panel on :$PANEL_PORT"
    local secret
    secret="$(openssl rand -hex 32)"
    local -a extra_env=()
    [ -n "${K3S_JOIN_URL:-}" ] && extra_env+=(K3S_JOIN_URL="$K3S_JOIN_URL")
    [ -n "${JOIN_KEY:-}" ] && extra_env+=(JOIN_KEY="$JOIN_KEY")
    ( cd "$RUNTIME_DIR" && setsid nohup env \
        JWT_SECRET="$secret" \
        NODES_DB="$NODES_DB" \
        DOWNLOADS_DB="$DOWNLOADS_DB" \
        SESSIONS_DB="$SESSIONS_DB" \
        CONTROL_PANEL_PORT="$PANEL_PORT" \
        ADVERTISE_MDNS=false \
        "${extra_env[@]}" \
        "$PANEL_BIN" > "$PANEL_LOG" 2>&1 < /dev/null & echo $! > "$PANEL_PID" )

    if ! wait_http "http://127.0.0.1:$PANEL_PORT/healthz" 15; then
        tail -20 "$PANEL_LOG" >&2 || true
        die "panel did not come up on :$PANEL_PORT (see $PANEL_LOG)"
    fi
    log "panel healthy: http://127.0.0.1:$PANEL_PORT/healthz"
}

start_trap() {
    if [ -f "$TRAP_PID" ] && kill -0 "$(cat "$TRAP_PID")" 2>/dev/null; then
        log "trap already running (pid $(cat "$TRAP_PID"))"
        return 0
    fi
    port_busy "$TRAP_PORT" && die "port $TRAP_PORT is already in use — set TRAP_PORT"

    log "starting trap on :$TRAP_PORT (advertising $AP_IP)"
    setsid nohup python3 "$TOOL_DIR/captive-trap.py" \
        --iface "$AP_IFACE" --panel-port "$PANEL_PORT" \
        --listen-port "$TRAP_PORT" \
        > "$TRAP_LOG" 2>&1 < /dev/null &
    echo $! > "$TRAP_PID"

    local deadline=$((SECONDS + 10))
    while [ "$SECONDS" -lt "$deadline" ]; do
        if curl -s -m 1 -o /dev/null -w '%{http_code}' \
            "http://127.0.0.1:$TRAP_PORT/" 2>/dev/null | grep -qx 302; then
            log "trap answering with 302: $(sed -n '1p' "$TRAP_LOG")"
            return 0
        fi
        sleep 0.2
    done
    tail -20 "$TRAP_LOG" >&2 || true
    die "trap did not answer on :$TRAP_PORT (see $TRAP_LOG)"
}

sudo_block() {
    cat <<EOF
──────────────────────── privileged setup (run these yourself) ────────────────────────
# 1. Onboarding AP — AP mode, pinned IP so NetworkManager adds no DHCP/DNS helper
#    (NM's internal dnsmasq cannot spawn on some hosts: "Operation not permitted").
nmcli connection show "$CON_NAME" >/dev/null 2>&1 || \\
  sudo nmcli connection add type wifi ifname "$AP_IFACE" con-name "$CON_NAME" \\
    autoconnect no ssid "$SSID" 802-11-wireless.mode ap \\
    802-11-wireless.band bg 802-11-wireless.channel 6 \\
    wifi-sec.key-mgmt wpa-psk wifi-sec.psk "$WIFI_PASSWORD" \\
    ipv4.method manual ipv4.addresses "$AP_IP/24"
sudo nmcli connection up "$CON_NAME"

# 2. DHCP + wildcard DNS (root; daemonizes, pid file in the runtime dir)
sudo dnsmasq --conf-file="$DNSMASQ_CONF"

# 3. Interception: hotspot port 80 -> trap :$TRAP_PORT (table is recreated)
sudo nft delete table ip ldndrc-captive 2>/dev/null || true
sudo nft -f "$NFT_FILE"

# Then join "$SSID" (password: $WIFI_PASSWORD) from the test laptop.
# Portal: http://$AP_IP:$PANEL_PORT/portal
───────────────────────────────────────────────────────────────────────────────────────
EOF
}

run_sudo() {
    [ -f "$NFT_FILE" ] || render_configs
    sudo_block
}

stop_all() {
    local pid
    for pid in "$PANEL_PID" "$TRAP_PID"; do
        [ -f "$pid" ] || continue
        if kill -0 "$(cat "$pid")" 2>/dev/null; then
            log "stopping $(basename "${pid%.pid}") (pid $(cat "$pid"))"
            kill "$(cat "$pid")" 2>/dev/null || true
        fi
        rm -f "$pid"
    done
    # Fallback for pid files lost to a crash: match only this runtime dir.
    pkill -f -- "$PANEL_BIN" 2>/dev/null || true
    pkill -f -- "$TOOL_DIR/captive-trap.py" 2>/dev/null || true

    cat <<EOF

Unprivileged processes stopped. Root-owned pieces are still up — remove them with:
  sudo nft delete table ip ldndrc-captive
  sudo kill "\$(cat $DNSMASQ_PID)" 2>/dev/null || true
  nmcli connection down "$CON_NAME" 2>/dev/null || true
EOF
}

status() {
    resolve_iface
    local hp tp
    hp="down"; tp="down"
    curl -fsS -m 2 -o /dev/null "http://127.0.0.1:$PANEL_PORT/healthz" 2>/dev/null && hp="up"
    [ "$(curl -s -m 2 -o /dev/null -w '%{http_code}' "http://127.0.0.1:$TRAP_PORT/" 2>/dev/null)" = "302" ] && tp="302"

    printf 'panel      : %-4s http://127.0.0.1:%s/healthz\n' "$hp" "$PANEL_PORT"
    printf 'trap       : %-4s http://127.0.0.1:%s/ (302 expected)\n' "$tp" "$TRAP_PORT"
    printf 'hotspot    : %s\n' "$(nmcli -t -f NAME,DEVICE,STATE connection show --active 2>/dev/null \
        | grep -F "$CON_NAME" || echo 'not active')"
    printf 'iface IPv4 : %s\n' "$(ip -4 -o addr show dev "$AP_IFACE" 2>/dev/null \
        | awk '{print $4}' || echo 'no IPv4')"
    printf 'dnsmasq    : %s\n' "$([ -f "$DNSMASQ_PID" ] && kill -0 "$(cat "$DNSMASQ_PID")" 2>/dev/null \
        && echo "running (pid $(cat "$DNSMASQ_PID"))" || echo 'not running')"
    if [ "$hp" = "up" ]; then
        printf 'counter    : %s\n' "$(curl -s -m 2 "http://127.0.0.1:$PANEL_PORT/api/downloads/count" 2>/dev/null || echo '-')"
    fi
    printf 'portal URL : http://%s:%s/portal\n' "$AP_IP" "$PANEL_PORT"
    printf 'nft table  : %s\n' "$(sudo -n nft list table ip ldndrc-captive >/dev/null 2>&1 \
        && echo 'present' || echo 'unknown (needs root to inspect)')"
}

case "${1:-}" in
    start)
        resolve_iface
        render_configs
        build_panel
        start_panel
        start_trap
        echo
        sudo_block
        ;;
    stop)   stop_all ;;
    status) status ;;
    config)
        resolve_iface
        printf 'AP_IFACE=%s\nAP_IP=%s\nPANEL_PORT=%s\nTRAP_PORT=%s\nRUNTIME_DIR=%s\n' \
            "$AP_IFACE" "$AP_IP" "$PANEL_PORT" "$TRAP_PORT" "$RUNTIME_DIR"
        printf 'portal=%s\n' "http://$AP_IP:$PANEL_PORT/portal"
        ;;
    sudo)   run_sudo ;;
    *)
        sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'
        exit 1
        ;;
esac
