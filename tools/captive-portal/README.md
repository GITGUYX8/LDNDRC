# Captive-portal rig (onboarding AP harness)

A complete, single-command demo of the LAN onboarding experience:

> connect to the `ldndrc-onboarding` Wi-Fi → the OS shows a "sign in to
> network" prompt → our portal opens → download the enrollment script →
> the instructor approves the laptop.

Everything is served by the master laptop. No internet, no cloud, no
cluster traffic on the onboarding radio.

**This is a workshop/demo harness, not gateway enforcement.** It
intercepts port-80 traffic and shows the portal; it does not block or
authorize clients at the network layer. Approval stays data in the panel;
enforcement at a router/gateway is the designed follow-up (contract in
[`docs/lan-onboarding-portal.md`](../../docs/lan-onboarding-portal.md)).

## What's in here

| File | Role |
|---|---|
| `run.sh` | `start` / `stop` / `status` / `config` / `sudo`. Builds the panel, renders configs, starts the unprivileged processes, and prints the privileged setup block. |
| `captive-trap.py` | Answers every intercepted request with `302 → <portal>`. Resolves the portal URL from the hotspot interface's IPv4 (or a fixed `--panel-url`). |
| `ldndrc-captive.nft.tmpl` | nftables NAT rule: `iifname <hotspot> tcp dport 80 redirect to :<trap>`. Interface-matched, so the subnet can change. |
| `dnsmasq-onboarding.conf.tmpl` | DHCP for joining laptops + wildcard DNS (`address=/#/<master-ip>`), so any `http://` attempt lands on the master. |

Runtime files (panel binary, JSON stores, logs, rendered configs) go to
`$RUNTIME_DIR` — default `~/.local/state/ldndrc-captive/`. Nothing is
written to `/etc` or committed.

## Quickstart

```bash
# 1. Start the unprivileged half (panel + trap) and print the root steps
./tools/captive-portal/run.sh start

# 2. Run the printed privileged block in another terminal (hotspot,
#    dnsmasq, nft). It is also available alone with:
./tools/captive-portal/run.sh sudo

# 3. From the test laptop: join "ldndrc-onboarding"
#    password: ldndrc-join-2026
#    → OS prompt → http://192.168.4.1:8082/portal

# 4. Tear down
./tools/captive-portal/run.sh stop      # unprivileged processes
# then the root teardown lines it prints
```

If a deployed control-panel pod already owns `:8082` (hostNetwork), run the
test panel elsewhere:

```bash
PANEL_PORT=8083 ./tools/captive-portal/run.sh start
# portal becomes http://192.168.4.1:8083/portal
```

The trap and the printed commands use the same port, so the redirects
follow automatically.

### Configuration (environment variables)

| Variable | Default | Meaning |
|---|---|---|
| `AP_IFACE` | auto-detected Wi-Fi iface | Hotspot radio (`wlp0s20f3`, `wlan0`, …) |
| `AP_IP` | `192.168.4.1` | Master's address on the onboarding network |
| `DHCP_START` / `DHCP_END` | `192.168.4.50` / `.150` | Lease range handed to laptops |
| `PANEL_PORT` | `8082` | Control-panel port (portal, counter, APIs) |
| `TRAP_PORT` | `8090` | Where the trap listens before nft redirect |
| `SSID` | `ldndrc-onboarding` | Network name laptops join |
| `WIFI_PASSWORD` | `ldndrc-join-2026` | WPA2 password (demo default — change for real use) |
| `CON_NAME` | `ldndrc-onboarding` | NetworkManager connection profile name |
| `RUNTIME_DIR` | `~/.local/state/ldndrc-captive` | Binary, stores, logs, rendered configs |
| `SKIP_BUILD` | `0` | `1` reuses an existing `$RUNTIME_DIR/panel` |
| `K3S_JOIN_URL`, `JOIN_KEY` | unset | Passed through to the panel when set |

## Why the hotspot is set up this way

The obvious recipe — NetworkManager's `ipv4.method=shared` — failed on the
reference laptop: NM's internal dnsmasq could not even be spawned
(`Failed to execute child process "/usr/bin/dnsmasq" (Operation not
permitted)`), surviving file capabilities, AppArmor checks and reboots.
The working arrangement splits responsibilities:

1. **NetworkManager** owns the radio only: `802-11-wireless.mode ap` plus
   `ipv4.method manual` / `192.168.4.1/24`, so NM runs no DHCP or DNS
   helper at all.
2. **Our dnsmasq** (run as root, not via capabilities) serves DHCP and
   wildcard DNS on that interface.
3. **nftables** rewrites port 80 arriving on the hotspot to the trap,
   which 302s to the portal.

The radio itself always came up fine; only NM's helper was the problem.

## What the joining laptop sees

```
join "ldndrc-onboarding"
  → DHCP 192.168.4.x, DNS = master, every name → 192.168.4.1
  → OS probe to http://connectivitycheck.gstatic.com/generate_204 (etc.)
  → nft REDIRECT → trap :8090 → 302 → http://192.168.4.1:8082/portal
  → welcome page, live download counter, joining-cluster.{sh,ps1}
```

On Omarchy/Hyprland the prompt surfaces as an nm-applet **notification**,
not a full-screen portal — one click opens the page. Windows shows the
classic "Sign in to network" banner.

## Verifying it works

```bash
./test-captive-trap.sh              # redirect contract, no root needed
./tools/captive-portal/run.sh status
curl -s http://127.0.0.1:8082/api/downloads/count
```

Expected on the master while the rig is up:

- `status` shows `panel up`, `trap 302`, hotspot active, `192.168.4.1/24`.
- `curl -I http://192.168.4.1/` from a hotspot client → `302` to the portal.
- The counter increments when a script finishes downloading, and updates
  live on the portal (polled every 3 s).

## Teardown / troubleshooting

```bash
# unprivileged
./tools/captive-portal/run.sh stop

# root pieces (also printed by stop)
sudo nft delete table ip ldndrc-captive
sudo kill "$(cat ~/.local/state/ldndrc-captive/dnsmasq.pid)" 2>/dev/null || true
nmcli connection down ldndrc-onboarding
```

| Symptom | Cause | Fix |
|---|---|---|
| `port 8082 is already in use` | deployed panel pod owns it (hostNetwork) | `PANEL_PORT=8083 ./run.sh start` |
| Clients get no IP | dnsmasq not running, or hotspot down first | start the hotspot, then `sudo dnsmasq --conf-file=…` |
| No captive prompt | nft rule missing, or client cached the network | `sudo nft list table ip ldndrc-captive`; "forget" and rejoin the Wi-Fi |
| Prompt appears, page does not | trap down / wrong panel port | `./run.sh status`; check `$RUNTIME_DIR/trap.log` |
| Redirect lands on the home-LAN IP | interface address changed | the trap re-reads it per request; confirm `ip -4 addr show <iface>` |
| `dnsmasq: failed to create listening socket for port 53` | the master's own resolver owns `:53` | stop systemd-resolved/`dnsmasq.service`, or add `bind-dynamic` (already set) and retry |
| Portal counter stuck | browser tab backgrounded | it polls every 3 s when visible; refresh once |

## Safety notes

- The trap redirects **all** port-80 traffic on the hotspot interface —
  that is the point, but do not run it on an interface that also carries
  trusted production traffic.
- `WIFI_PASSWORD` is a demo default printed in this file. Change it with
  the environment variable for any real workshop, and change the panel's
  `JOIN_KEY` if you rely on it.
- The AP radio and the home Wi-Fi share one card (same channel). Throughput
  on both drops while the rig is up; stop the hotspot when finished.
