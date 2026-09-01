# Zero-Touch Host Onboarding — Design Report (For Review)

**Status:** Proposal — awaiting review before implementation
**Date:** 2026-08-21
**Scope:** `join-cluster.sh` replacement, `control-panel/` extension, master-laptop operations

---

## 1. Executive Summary

Today, adding a laptop to the LDNDRC cluster as a **Host** requires manual steps:
typing the master's IP, pasting a K3s node token, installing the K3s agent by
hand, installing the NVIDIA container toolkit, and guessing whether any of it
worked. This report proposes a **zero-touch onboarding mechanism**:

> A person runs **one command** on a qualifying laptop. The machine finds the
> master on the LAN by itself, requests to join, waits for an operator to click
> **Approve** on a web dashboard, then installs and verifies everything
> automatically.

The mechanism has two halves:

1. **`ldndrc-join`** — a single cross-platform Go binary (Linux / Windows /
   macOS) that runs on the *joining* laptop.
2. **Control panel extensions** — the existing Go control panel gains mDNS
   advertisement, a node-registration API, short-lived token minting, a node
   watcher, and an approval dashboard. It runs as a **systemd unit on the
   master laptop**.

No simulation harness is included in this iteration (decision §4.1).

---

## 2. Background & Current State

### 2.1 What exists today

| Asset | Role | Relevant detail |
|---|---|---|
| `join-cluster.sh` | Hardware gatekeeper + K3s agent installer | Detects CPU (`nproc`), RAM (`/proc/meminfo`), GPU (`lspci`); requires `MASTER_IP` and `NODE_TOKEN` env vars (L15–16); installs agent with `--node-label role=host` (L44–45) |
| `test-join.sh` | Regression tests for the detection logic | 4 scenarios via `TEST_CPU/TEST_RAM/TEST_GPU` overrides, ~2s, no cluster needed |
| `manifests/ros2-platform.yaml` | StatefulSet, 3 replicas | Pods target `node-role.kubernetes.io/role: host` nodes via nodeSelector |
| `manifests/ros2-service.yaml` | ClusterIP Service | ClientIP session affinity (1h), ports 7682/8080/9002 |
| `manifests/ros2-ingress.yaml` | Traefik ingress | Routes `/code`, `/stream`, `/sim` on `ros-platform.local` |
| `control-panel/` | Go session/auth backend | JWT service (`internal/auth`), router with `bearerClaims` middleware (`internal/httpapi/router.go:75`), sessions store seam (not yet wired), gateway proxy, **empty `web/` dir**, single dep `golang-jwt/jwt/v5` |

### 2.2 The manual steps being eliminated

| # | Manual step today | Pain |
|---|---|---|
| 1 | Look up / type `MASTER_IP` | Operator must communicate the IP out-of-band |
| 2 | Paste `NODE_TOKEN` (a long, powerful secret) | Error-prone, and the static token is all-powerful |
| 3 | Run the k3s agent install manually | Requires shell familiarity |
| 4 | Ensure `nvidia-container-toolkit` is installed | Easy to forget; failure surfaces only when a GPU pod starts |
| 5 | Verify the node joined, got labeled, and is Ready | No feedback loop — pure guesswork |

### 2.3 Why this matters to the project

LDNDRC's premise (plan.md §1) is that the cluster grows by *whoever's laptop
happens to be in the room*. Every manual step filters out non-experts and
slows cluster formation. Zero-touch onboarding is what makes the "dynamic node
discovery" in the project's name real.

---

## 3. Goals and Non-Goals

### 3.1 Goals

- **G1.** One command on the joining laptop: `ldndrc-join` (double-clickable later).
- **G2.** No pre-shared master IP — automatic LAN discovery via mDNS.
- **G3.** No pre-shared long-lived token — short-lived tokens minted **after** human approval.
- **G4.** Human approval gate: an operator sees each join request and approves/denies it.
- **G5.** Automatic post-join verification: the laptop knows it succeeded (or exactly why it failed).
- **G6.** Cross-platform detection: the same binary runs on Linux, Windows, macOS; hardware thresholds measured on **physical** hardware.
- **G7.** Minimal new dependencies; the control panel stays a single static binary.

### 3.2 Non-goals (this iteration)

- **N1.** No rehearsal/simulation harness (`machinesim`) — production mechanism only.
- **N2.** No automatic NVIDIA **driver** install — toolkit only; missing driver = clear error.
- **N3.** No TLS on the control panel API — LAN trust assumed; flagged for later hardening.
- **N4.** No automatic WSL2 installation on Windows — guided handoff only (requires admin + reboot).
- **N5.** No changes to `ros2-*.yaml` manifests — onboarding is orthogonal to the workload.
- **N6.** `join-cluster.sh` / `test-join.sh` remain untouched as the legacy Linux path until the Go binary is proven on real hardware.

---

## 4. Decision Log

These decisions were made interactively during planning and are baked into the design.

| # | Question | Decision | Alternatives rejected |
|---|---|---|---|
| 4.1 | Scope | **Production onboarding only** | Simulation harness first; both |
| 4.2 | Discovery & auth | **mDNS auto-discover + operator approval** | QR code; full auto-approve; manual-only |
| 4.3 | GPU handling | **Detect + install toolkit only** | Full driver auto-install; check-only |
| 4.4 | Where the service lives | **Extend existing Go control panel** | Separate Go service; shell+kubectl only |
| 4.5 | Token strategy | **Short-lived minted tokens** (`k3s token create --ttl 15m`), minted lazily at first poll after approval | Static node token from `/var/lib/rancher/k3s/server/token` |
| 4.6 | Windows host support | **Cross-platform Go binary** (`ldndrc-join`) | Linux-only documented; experimental WSL2-only path |
| 4.7 | Control panel runtime | **systemd unit on the master** | In-cluster Deployment; split duties |

### 4.1 Review corrections already applied to this design

During plan review, five defects in earlier drafts were found and fixed:

1. **mDNS library:** the initial pick (`grandcat/zeroconf`) is archived/unmaintained. Replaced with **`hashicorp/mdns`** (actively maintained; used for both advertising on the master and browsing on the laptop).
2. **Script naming:** an earlier draft introduced a new `onboard.sh`, which would break `plan.md` references and `test-join.sh`. Final design keeps `join-cluster.sh` untouched and introduces the Go binary as the new path.
3. **Node-name mapping gap:** a pending registration must map deterministically to the k3s node that later appears. Relying on hostnames risks collisions (two laptops both named `ubuntu`). Fix: the agent joins with **`--node-name ldndrc-<id>`**, so the watcher matches exactly.
4. **Token TTL race:** minting the token at approve-time starts the TTL clock; a slow k3s download could outlive it. Fix: **lazy minting** — approve sets `status=approved`; the token is minted fresh when the laptop polls, immediately before use.
5. **Dependency weight:** SQLite and `client-go` were proposed; both are overkill for v1. Replaced with a **JSON-file store** (mutex + atomic write, cgo-free) and **`k3s kubectl` exec** for node watching/labeling (client-go is deferred to the Phase-5 sessions work, where plan.md already intends it).

---

## 5. Architecture Overview

```
 JOINING LAPTOP (any OS)                       MASTER LAPTOP (Linux)
 ┌─────────────────────────────┐              ┌──────────────────────────────────┐
 │ ldndrc-join (Go binary)     │              │ systemd: ldndrc-control-panel    │
 │                             │              │                                  │
 │ 1. OS gate                  │              │  A. mDNS advertise               │
 │ 2. detect CPU/RAM/GPU       │  ① mDNS      │     _ldndrc-master._tcp          │
 │    (physical hardware)      │◄─────────────│     TXT {port, cluster}          │
 │ 3. browse _ldndrc-master._tcp│             │                                  │
 │ 4. POST /api/nodes/register │──② HTTP────► │  B. nodes API (register/poll/    │
 │    {hostname,os,cpu,ram,gpu}│              │     approve/deny) + JSON store   │
 │ 5. poll GET /api/nodes/{id} │──③ poll────► │                                  │
 │    every 5s, 15 min timeout │              │  C. web dashboard: operator      │
 │                             │              │     reviews → Approve / Deny     │
 │ 6. on approved: receive     │◄──④ token─── │  D. lazy mint:                   │
 │    {k3s_url, k3s_token,     │              │     k3s token create --ttl 15m   │
 │     node_name}              │              │                                  │
 │ 7. install toolkit (if GPU) │              │  E. watcher: k3s kubectl get     │
 │ 8. install k3s agent        │──⑤ 6443────► │     node ldndrc-<id> → label     │
 │    --node-name ldndrc-<id>  │   k3s API    │     role=host → status=joined    │
 │ 9. poll until status=joined │──⑥ poll────► │     (GPU Operator DaemonSets     │
 │ 10. print success + URLs    │              │      auto-deploy device plugin)  │
 └─────────────────────────────┘              └──────────────────────────────────┘
        Windows laptop only: 7b/8b hand off into WSL2 (guided)
        Non-qualifying laptop: guest mode — print browser URLs, exit 0
```

### 5.1 End-to-end sequence

| Step | Actor | Action |
|---|---|---|
| ① | master | Advertises itself continuously via mDNS (UDP 5353) |
| ② | laptop | Detects hardware; if Host-qualified, discovers master, POSTs fingerprint; receives `id` |
| ③ | laptop + operator | Laptop polls every 5s and prints *"waiting for operator approval at http://\<master\>:8082"*. Operator logs into the dashboard, sees the request with hardware details, clicks **Approve** |
| ④ | master → laptop | On the laptop's first poll after approval, the control panel execs `k3s token create --ttl 15m --description ldndrc-<id>` and returns `{k3s_url, k3s_token, node_name}` |
| ⑤ | laptop | Installs NVIDIA container toolkit if needed (sudo), then installs the k3s agent against the master's API (TCP 6443) |
| ⑥ | master | Watcher sees node `ldndrc-<id>` become Ready, applies the `role=host` label, marks the record `joined`; the laptop's final poll confirms and prints the success message + removal instructions |

### 5.2 Why approval (not auto-approve) is the default

An unauthenticated registration endpoint is acceptable only because it creates
a *pending request*, never a credential. The sensitive artifact — a cluster
join token — is gated behind a human decision. An `AUTO_APPROVE=true` env flag
is included for demos but defaults off.

---

## 6. Component Design — Control Panel (master side)

### 6.1 New packages

```
control-panel/
  cmd/server/main.go            — wire discovery + watcher + nodes API (modified)
  internal/
    auth/                       — unchanged; bearerClaims reused for operator routes
    discovery/advertise.go      — NEW: hashicorp/mdns advertiser
    nodes/store.go              — NEW: JSON-file store (idempotent, atomic)
    nodes/handlers.go           — NEW: HTTP handlers
    nodes/tokens.go             — NEW: lazy k3s token minting
    nodes/watcher.go            — NEW: node watch + labeling via k3s kubectl
    httpapi/router.go           — modified: mount nodes routes
  web/                          — NEW: embedded approval dashboard (currently empty dir)
  packaging/
    ldndrc-control-panel.service — NEW: systemd unit
```

### 6.2 `internal/discovery` — mDNS advertisement

- Advertises service type `_ldndrc-master._tcp`, instance name from hostname,
  port = control panel port (default 8082).
- TXT records: `cluster=ldndrc`, `version=1`, `path=/api/nodes/register`.
- Started in `main.go` only when `ADVERTISE_MDNS=true` (default true on the master).
- Implementation: `hashicorp/mdns.NewMDNSService` + `mdns.NewServer` — a few
  dozen lines, no avahi dependency on the master.

### 6.3 `internal/nodes/store.go` — persistence

```jsonc
// $NODES_DB (default /var/lib/ldndrc/nodes.json)
{
  "nodes": {
    "b7f3": {
      "id": "b7f3",
      "hostname": "priya-laptop",
      "os": "linux", "arch": "amd64",
      "cpu": 12, "ram_gb": 32, "gpu": "NVIDIA GeForce RTX 4060",
      "status": "approved",            // pending|approved|denied|joined|expired
      "node_name": "ldndrc-b7f3",
      "requested_at": "2026-08-21T10:04:11Z",
      "approved_at": "2026-08-21T10:05:02Z",
      "token_minted": false
    }
  }
}
```

- In-process `sync.RWMutex`; writes via temp-file + `rename` (atomic).
- **Idempotent register:** keyed by `hostname + fingerprint hash` — re-running
  `ldndrc-join` returns the existing record instead of creating duplicates.
- JSON chosen over SQLite: a handful of nodes, no queries beyond by-ID/list,
  keeps the binary cgo-free, zero new heavy deps.

### 6.4 `internal/nodes/tokens.go` — lazy token minting

- Trigger: first `GET /api/nodes/{id}` poll where `status == approved` and
  `token_minted == false`.
- Executes: `k3s token create --ttl 15m --description ldndrc-<id>` on the
  master; parses stdout; marks `token_minted = true`.
- The token is returned **once** and not persisted in the store (the response
  struct carries it; the JSON file does not).
- TTL 15m comfortably covers the k3s installer download; if it expires, the
  operator re-approves (status reset) — an explicit, logged operation.
- Requires the control panel to run with permission to exec `k3s` — satisfied
  by the systemd unit (root on the master).

### 6.5 `internal/nodes/watcher.go` — join verification & labeling

- Every 5s, for each node in `approved` state: exec
  `k3s kubectl get node ldndrc-<id> -o json`.
- When the node exists and `Ready`:
  - `k3s kubectl label node ldndrc-<id> node-role.kubernetes.io/role=host ldndrc/host=true --overwrite`
  - set `status = joined`.
- Exec-based (not client-go) to keep dependencies light for v1; the seam is a
  small interface so client-go can replace it when Phase-5 sessions lands.
- Belt-and-suspenders: the agent *also* self-labels via `--node-label`, so
  labeling succeeds even if the watcher is briefly down.

### 6.6 `internal/httpapi/router.go` — route wiring

Existing routes (`GET /healthz`, `POST /api/auth/login`, optional sessions)
are untouched. Added:

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/api/nodes/register` | none (optional `JOIN_KEY`) | create pending record, return `{id}` |
| GET | `/api/nodes/{id}` | none (knowledge of `id`) | laptop polls status; returns token payload once when approved |
| GET | `/api/nodes` | JWT (operator) | list all nodes with status/hardware |
| POST | `/api/nodes/{id}/approve` | JWT (operator) | approve; token minted lazily on next poll |
| POST | `/api/nodes/{id}/deny` | JWT (operator) | deny with optional reason |

Notes:
- The unauthenticated `register` endpoint gets a per-IP rate limit
  (e.g., 10/min) and an optional shared `JOIN_KEY` env — cheap spam control
  on the LAN.
- `GET /api/nodes/{id}` exposes no secrets until `approved`, and the token is
  returned exactly once.

### 6.7 `web/` — operator dashboard

`web/` is currently an empty directory, so this is greenfield:

- Single embedded page (`embed.FS`), consistent with plan.md Phase 5.
- Login (existing JWT endpoint) → **Pending requests** table (hostname, OS,
  CPU/RAM/GPU, time) with Approve/Deny buttons → **Joined nodes** table with
  Ready status.
- Plain HTML + fetch + tiny JS; no framework — matches the "single static
  binary" constraint.

### 6.8 `packaging/ldndrc-control-panel.service` — systemd unit

```ini
[Unit]
Description=LDNDRC control panel (discovery, onboarding, sessions)
After=network-online.target k3s.service
Wants=network-online.target

[Service]
Environment=JWT_SECRET=__set_me__ ADVERTISE_MDNS=true NODES_DB=/var/lib/ldndrc/nodes.json
ExecStart=/usr/local/bin/ldndrc-control-panel
Restart=on-failure
User=root                      # required to exec `k3s token create` / `k3s kubectl`

[Install]
WantedBy=multi-user.target
```

**Why systemd and not an in-cluster Deployment** (decision 4.7):

| | systemd on master | In-cluster Deployment |
|---|---|---|
| mDNS advertise | native (host network) | needs `hostNetwork: true` |
| `k3s token create` / `k3s kubectl` | direct exec | must mount k3s binary + server state into the pod — fragile |
| Cluster down / bootstrapping | still alive — can onboard the first node | dies with the cluster it onboards into |
| Ops model | `systemctl`, journald | `kubectl`, matches Phase 5 deployment style |

The Phase-5 session API can still become an in-cluster Deployment later
("split" end-state); onboarding duties stay host-level.

---

## 7. Component Design — `ldndrc-join` (laptop side)

One static Go binary per platform, built from `control-panel/cmd/join/`
(same module — shares `hashicorp/mdns`; the linker strips the rest).

### 7.1 Execution flow

```
ldndrc-join
  │
  ├─ 1. OS gate (runtime.GOOS)
  │     darwin  → guest mode (no NVIDIA/k3s on macOS)
  │     windows → detect physical hardware (Win APIs) → qualify? WSL2 handoff : guest mode
  │     linux   → continue
  │
  ├─ 2. Hardware detection (build-tagged)
  │     CPU: runtime.NumCPU()
  │     RAM: linux /proc/meminfo · windows GlobalMemoryStatusEx · darwin sysctl hw.memsize
  │     GPU: exec `nvidia-smi -L`  (works on Linux, Windows, and inside WSL2)
  │     thresholds unchanged: ≥8 cores, ≥16 GB, NVIDIA present  → else guest mode
  │
  ├─ 3. Discover master
  │     browse _ldndrc-master._tcp via hashicorp/mdns client (≈3s timeout)
  │     fallback: -master flag / MASTER_IP env (kept from join-cluster.sh)
  │
  ├─ 4. Register: POST /api/nodes/register {hostname,os,arch,cpu,ram_gb,gpu}
  │     → {id}; idempotent on re-run
  │
  ├─ 5. Poll GET /api/nodes/{id} every 5s, 15 min timeout
  │     pending → print "waiting for operator approval at http://<master>:8082"
  │     denied  → print reason, exit 1
  │
  ├─ 6. Approved → receive {k3s_url, k3s_token, node_name}
  │
  ├─ 7. GPU toolkit (Linux, NVIDIA only)
  │     nvidia-smi missing        → clear error (no driver auto-install — N2)
  │     nvidia-container-toolkit? → apt install (sudo prompt) + nvidia-ctk runtime configure
  │
  ├─ 8. Install k3s agent
  │     curl -sfL https://get.k3s.io | K3S_URL=… K3S_TOKEN=… \
  │       sh -s - agent --node-name ldndrc-<id> \
  │                     --node-label node-role.kubernetes.io/role=host
  │
  ├─ 9. Poll until status=joined (watcher confirms Ready + label)
  │
  └─ 10. Print success, URLs, and removal note (k3s-uninstall.sh)
```

### 7.2 Windows specifics (decision 4.6)

- Detection runs **natively** (Win APIs), so thresholds measure the **physical
  laptop** — this fixes the "detection lies inside a VM" caveat of running the
  bash script inside WSL2.
- On qualify: if WSL2 is present (`wsl --status`), the binary copies the Linux
  build into the WSL filesystem and execs it there (steps 3–9 rerun inside).
- If WSL2 is absent: prints guided steps — `wsl --install`, reboot, enable
  **mirrored networking** (Windows 11 22H2+; required so flannel VXLAN UDP
  8472 is reachable inbound). No silent auto-install (N4).

### 7.3 Package layout

```
control-panel/cmd/join/main.go
control-panel/internal/join/
  hardware.go            — types + threshold logic (pure, unit-tested)
  hardware_linux.go      — /proc/meminfo
  hardware_windows.go    — GlobalMemoryStatusEx via golang.org/x/sys/windows
  hardware_darwin.go     — sysctl
  discover.go            — mDNS browse + MASTER_IP fallback
  register.go            — register/poll HTTP client
  install_linux.go       — toolkit + k3s agent install
  handoff_windows.go     — WSL2 detection + handoff
```

New dependencies: `hashicorp/mdns` (shared with the server) and
`golang.org/x/sys` (Windows/darwin syscalls). Both are small and maintained.

### 7.4 Distribution (the chicken-and-egg problem)

The binary can't be downloaded from a master it hasn't discovered yet. v1
options, in order of preference: GitHub Release artifacts, USB stick, or
`served from the repo` during workshops. This is a **documentation/packaging
task**, not code.

---

## 8. Node Lifecycle State Machine

```
                 register                 approve                 watcher: Ready+labeled
   (none) ────────────────► pending ────────────────► approved ──────────────────────► joined
                               │                        │
                               │ deny                   │ token TTL expires unused
                               ▼                        ▼
                             denied                  expired ──► operator re-approves ──► approved
```

- `pending` — record created; visible on the dashboard.
- `approved` — operator decision made; token **not yet minted**.
- `joined` — node is Ready and labeled; workloads can schedule (nodeSelector `role=host`).
- `denied` / `expired` — terminal-ish; re-registration or re-approval required.

---

## 9. Security Model

| Threat | Mitigation |
|---|---|
| Random LAN device floods register endpoint | Per-IP rate limit; optional `JOIN_KEY` shared secret |
| Unauthorized laptop joins cluster | Human approval gate; token minted only after approval |
| Token leakage/replay | `k3s token create --ttl 15m` — short-lived, single-purpose; returned exactly once, never persisted to disk |
| Token in transit over HTTP | Accepted for v1 (LAN); TLS/mTLS listed as future hardening (N3) |
| Hostname collisions | Deterministic `--node-name ldndrc-<id>`; idempotent register keyed on hostname+fingerprint |
| Dashboard access | Existing JWT auth (`internal/auth`) guards operator endpoints |
| Privilege scope on master | systemd unit runs as root only because `k3s token create`/`k3s kubectl` require it; documented in the unit file |

---

## 10. OS Support Audit (as built today)

| Component | OS-specific bits | Verdict |
|---|---|---|
| Guest browser path (`/code`, `/stream`, `/sim`) | none — HTTP/WebSocket/WebRTC | ✅ OS-agnostic already; `ros-platform.local` resolves via mDNS on Win10+ and Bonjour on macOS |
| `join-cluster.sh` | `nproc` (L21), `/proc/meminfo` (L26), `lspci` (L32), bash, `curl\|sh` k3s install | ❌ Linux-only — **superseded by `ldndrc-join`** |
| `test-join.sh` | bash | ⚠️ dev tool, Linux/macOS; role replaced by Go unit tests (§12) |
| `images/*/installs.sh` | `apt-get` | ✅ runs *inside* Linux containers; host-OS irrelevant |
| Control panel | Go stdlib + jwt | ✅ portable; runs on the master (Linux) |
| k3s server + agent | — | ❌ Linux-only, officially (hard constraint) |
| NVIDIA container toolkit | — | ❌ Linux-only (except via WSL2) |

**Conclusion:** guests were already OS-agnostic; this design makes *detection
and joining* cross-platform, while honestly routing Windows hosts through WSL2
and macOS laptops to guest mode.

---

## 11. Alternatives Considered

| Area | Chosen | Rejected | Why |
|---|---|---|---|
| mDNS library | `hashicorp/mdns` | `grandcat/zeroconf` | zeroconf is archived/unmaintained |
| Token issuance | lazy `k3s token create --ttl 15m` | static node token from `/var/lib/rancher/k3s/server/token` | static token is long-lived and all-powerful; minted tokens expire and are single-purpose |
| Mint timing | on first poll after approval | at approve time | avoids TTL expiry during slow installer downloads |
| Storage | JSON file + atomic rename | SQLite (`modernc.org/sqlite`) | a handful of nodes; no query needs; keeps cgo out |
| Node watch/label | exec `k3s kubectl` | `client-go` | heavy dep tree; Phase 5 will bring client-go for sessions anyway — one migration, not two |
| Control panel runtime | systemd on master | in-cluster Deployment | mDNS + k3s CLI + bootstrap independence (see §6.8) |
| Windows hosts | Go binary + WSL2 handoff | bash-in-WSL2 | bash-in-WSL2 detects the **VM's** vCPUs/RAM, not the laptop's |
| Node naming | `ldndrc-<id>` | laptop hostname | collisions (multiple `ubuntu`) would break the pending→joined mapping |
| Approval UX | web dashboard | CLI-only approval | operator may be non-technical; a browser click is the streamlining goal |

---

## 12. Testing Strategy

| Layer | Test | How |
|---|---|---|
| Threshold logic | Go unit tests (`hardware_test.go`) | fake CPU/RAM/GPU inputs — replaces `test-join.sh`'s role |
| Store | Go unit tests | idempotent register, atomic writes, state transitions |
| Handlers | Go `httptest` | register → poll → approve → token-minted-once; deny path; rate limit |
| Token minting | fake `exec` seam | asserts `k3s token create` args; TTL flag; single mint |
| Watcher | fake `exec` seam | Ready → label applied → `joined` |
| `ldndrc-join` flow | integration test against `httptest` server | full happy path without a cluster |
| Legacy path | `test-join.sh` | unchanged, still 4/4 |
| Real hardware | manual | one real Linux laptop joining a real master — the acceptance test |

---

## 13. Implementation Phases

| Phase | Deliverable | Depends on |
|---|---|---|
| **P1** | `internal/nodes` (store + handlers + router wiring) + `web/` dashboard skeleton | — |
| **P2** | `internal/discovery` mDNS advertise + `packaging/*.service` + master install docs | P1 |
| **P3** | `internal/nodes/tokens.go` + `watcher.go` (exec seams + unit tests) | P1 |
| **P4** | `cmd/join` Linux path end-to-end (detect → discover → register → poll → install) + unit tests | P2, P3 |
| **P5** | Windows build: native detection + WSL2 handoff; darwin guest mode | P4 |
| **P6** | Docs: `plan.md` Phase 1 update, `local_dev_docs/` runbook (join/approve/exit), deprecation note for `join-cluster.sh` | P4 |

P1–P3 are master-side and testable without any joining laptop (httptest +
fake exec). P4 is the first end-to-end milestone.

---

## 14. Risks & Mitigations

| Risk | Likelihood | Mitigation |
|---|---|---|
| mDNS blocked by AP isolation / managed Wi-Fi | Medium | `MASTER_IP` fallback flag/env kept; runbook documents it |
| UDP 5353/8472 or TCP 6443 blocked by laptop firewall (ufw) | Medium | `ldndrc-join` preflights `master:6443` reachability and prints exact `ufw allow` lines |
| Token TTL expires before install finishes | Low | 15m TTL + mint-at-poll; re-approve path documented |
| Operator never approves (walked away) | Medium | 15-min poll timeout with a clear "still waiting / re-run later" message; request persists |
| WSL2 mirrored networking unavailable (older Windows) | Medium | detected; graceful guest-mode fallback with explanation |
| Root requirement of the systemd unit | Low | justified (k3s CLI), documented; audit via journald |
| Binary distribution friction (no master yet discovered) | Medium | GitHub Releases / USB; listed as packaging task in P6 |

---

## 15. Limitations & Future Work

**Accepted v1 limitations** (non-goals §3.2): no TLS on the API, no driver
auto-install, no silent WSL2 install, no machinesim harness, legacy bash path
kept in place.

**Natural next steps after this lands:**

1. Phase-5 sessions: replace exec-based watcher with `client-go`, dynamic
   per-session pod provisioning (replaces the fixed `replicas: 3` StatefulSet).
2. TLS/mTLS or an OIDC proxy in front of the control panel.
3. QR-code join UX (dashboard shows QR with master address) — the discovery
   fallback for mDNS-hostile networks.
4. MachineSim harness (the deferred option from decision 4.1) once real
   hardware proves the flow.
5. Deprecate `join-cluster.sh` / `test-join.sh` after the Go binary is proven.

---

## 16. Appendix

### 16.1 Environment / configuration reference

| Variable | Where | Default | Purpose |
|---|---|---|---|
| `JWT_SECRET` | control panel | — (required) | operator auth signing key |
| `ADVERTISE_MDNS` | control panel | `true` | enable `_ldndrc-master._tcp` advertisement |
| `NODES_DB` | control panel | `/var/lib/ldndrc/nodes.json` | node store path |
| `JOIN_KEY` | control panel | empty (off) | optional shared secret required at register |
| `AUTO_APPROVE` | control panel | `false` | demo mode: approve requests automatically |
| `CONTROL_PANEL_PORT` | control panel | `8082` | API/dashboard port |
| `MASTER_IP` | `ldndrc-join` | empty | skip mDNS, use this master address |

### 16.2 Firewall reference (LAN)

| Port | Proto | Direction | Why |
|---|---|---|---|
| 5353 | UDP | master ↔ laptops | mDNS discovery |
| 8082 | TCP | laptops → master | control panel API + dashboard |
| 6443 | TCP | laptops → master | k3s API (agent join) |
| 8472 | UDP | node ↔ node | flannel VXLAN pod networking |

### 16.3 API examples

```bash
# laptop → register
curl -X POST http://192.168.1.50:8082/api/nodes/register \
  -d '{"hostname":"priya-laptop","os":"linux","arch":"amd64","cpu":12,"ram_gb":32,"gpu":"NVIDIA GeForce RTX 4060"}'
# → {"id":"b7f3"}

# laptop → poll
curl http://192.168.1.50:8082/api/nodes/b7f3
# pending:  {"status":"pending"}
# approved: {"status":"approved","k3s_url":"https://192.168.1.50:6443","k3s_token":"K10…::server:…","node_name":"ldndrc-b7f3"}
# joined:   {"status":"joined"}

# operator → approve (JWT)
curl -X POST http://192.168.1.50:8082/api/nodes/b7f3/approve \
  -H "Authorization: Bearer <token>"
```

---

*Review checklist: goals (§3) · decisions (§4) · API surface (§6.6, §16.3) ·
security model (§9) · phases (§13). Comments welcome on any section; the plan
is implementation-ready pending your sign-off.*
