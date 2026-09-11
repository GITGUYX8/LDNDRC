# LDNDRC — Detailed Project Report

**Date:** 2026-09-11 · **Branch:** `main` · **Cluster:** k3d `ldndrc` (K3s v1.35.5)
**Releases:** `v0.1.0`, `v0.2.0` (`ldndrc-join`, 5 platform artifacts + SHA256SUMS)

This is the technical companion to the 19 checkpoint stories in
`local_dev_docs/checkpoint-*.md`. Where those tell what happened, this
documents what exists, what was proven and how, what is deliberately
provisional, and exactly what remains.

---

## 1. Project identity

LDNDRC (Local Dynamic Node Discovery ROS Cluster) is a local-first,
browser-accessible robotics simulation platform: consumer laptops form a K3s
cluster, strong laptops ("Hosts") run isolated per-user ROS 2 workspaces,
weak laptops ("Guests") use a browser. No cloud, no dedicated servers.

## 2. Architecture as built

```text
Guest browser / joining laptop (LAN)
  │  editor|desktop|gazebo|control .ros-platform.local → Traefik :80 (k3d lb)
  ▼
Control panel (Go, in-cluster Deployment, non-root distroless)
  ├─ /api/auth/*          JWT login + HttpOnly session cookie
  ├─ /api/sessions/*      per-user workspace CRUD (owner-scoped)
  ├─ /api/nodes/*         Host onboarding: register/poll/approve/deny/list/diagnostics
  └─ gateway              cookie-authenticated host routing → session Services
K3s API (client-go): per-session Deployment + Service + home PVC;
  bootstrap-token Secrets (15m TTL); node labeling; readiness reads
Headlamp (in-cluster, port-forward/token access): operator visuals
```

Key design facts: one Deployment + Service + PVC per user session with a
unique `ROS_DOMAIN_ID` and `ROS_AUTOMATIC_DISCOVERY_RANGE=LOCALHOST`;
Host-only scheduling via `node-role.kubernetes.io/role`; GPU requests
optional (`SESSION_GPU_LIMIT=0` on the GPU-less demo cluster, `"1"` default);
cookie auth for browsers, bearer tokens for API clients.

## 3. Work inventory

### 3.1 Control panel backend (`control-panel/`, Go 1.26.5)

| Package | Responsibility | State |
|---|---|---|
| `cmd/server` | Wiring: auth, stores, minter, watcher, gateway; `ADVERTISE_MDNS` gate | Done, live |
| `internal/auth` | JWT issue/verify, session cookie set/verify/clear | Done, live |
| `internal/httpapi` | Session CRUD + nodes API + diagnostics upload (1MB cap, rate-limited register, `JOIN_KEY`, `AUTO_APPROVE`) | Done, live |
| `internal/sessions` | In-memory ownership store + client-go provisioner (Deployment/Service/PVC, ROS env, GPU-conditional, probes, security contexts) | Done, live |
| `internal/nodes` | Join-request store (JSON file, atomic writes, idempotent register, approve/deny/expire state machine) | Done, live |
| `internal/nodes` tokens | Bootstrap-token Secrets via client-go, `K10…` secure format from cluster CA, mint-once semantics | Done, live |
| `internal/nodes` watcher | 5s tick: labels Ready nodes `role=host`, marks joined | Done, live |
| `internal/gateway` | Host-based reverse proxy (editor 7682 / desktop 8080 / gazebo 9002), auth-before-routing, 401/404/503/502 mapping | Done, live |
| `internal/discovery` | mDNS `_ldndrc-master._tcp` advertisement + TXT payload | Done, unit-tested; LAN proof needs a real master |
| `internal/join` | Onboarding engine: thresholds, per-OS detection (`TEST_*` overrides), mDNS browse + `MASTER_IP` fallback, register/poll client, consent-gated installer, diagnostics bundle | Done, unit-tested |
| `cmd/join` + TUI | Bubble Tea Checks/Fix/Confirm/Join/Done screens, `--plain` byte-parity mode, single-launch engine glue | Done, cross-compiles 3 OSes |

### 3.2 Manifests (`manifests/`)

RBAC (least-privilege: session resources in `ldndrc`, bootstrap secrets in
`kube-system`, nodes get/watch/patch cluster-wide), control-panel
Deployment/Service/Ingress (4 hostnames), Secret example (never commit
real values), Headlamp stack, hostNetwork patch (real masters only),
demo-standin + GPU-limit demo overrides (commented as such).

### 3.3 Join distribution

`.github/workflows/release-join.yml`: tag-triggered matrix
(linux/windows/darwin × amd64/arm64), vet+test gate, `SHA256SUMS`, auto
notes. `v0.1.0` (unwired Join screen) superseded by `v0.2.0` (current).

### 3.4 Docs

`plan.md` (original design), `docs/roadmap.md` (done/remaining map through
R5), `docs/local-cluster-headlamp.md` (full local runbook),
`docs/host-joiner-tui-spec.md` (TUI decisions D1–D4),
`onboarding-design-report.md` (amended for in-cluster reality),
`control-panel/README.md` (deploy + nodes API), 19 checkpoints.

## 4. Verification ledger

### Unit (all `ok`: gateway, httpapi, nodes, sessions, join, discovery)

Ownership isolation, duplicate prevention, lifecycle transitions,
idempotent register, approve/deny/expiry/re-approve, token-once minting,
watcher Ready/unready/missing cases, gateway 401/503 paths, qualify gates
×9, driver bands, `--plain` oracle strings, TUI keypress transitions,
bundle upload round-trip + 413 cap, announce-then-find mDNS round-trip.

### Live on k3d (observed, not simulated)

- Rollouts healthy; `/healthz` 200; RBAC probes correct (session rights
  yes, node reads no for the base role; minter/watcher rights yes).
- Two-user sessions: separate Deployment/Service/PVC, ROS domains 100/101,
  owner-scoped lists; deletion removes all three resources.
- Scheduler enforcement: unlabeled node → affinity refusal; GPU request on
  GPU-less node → `Insufficient nvidia.com/gpu`; GPU-optional revision
  schedules, binds PVC, pulls image.
- Gateway via Traefik: editor 200 (stand-in body), desktop/gazebo 502
  (nothing listening — correct), no-cookie 401, unready 503, old `/code`
  path 404 after R1 removal.
- Onboarding across real Wi-Fi (checkpoint-19): second-laptop register →
  approve → first poll minted `bootstrap-token-*` (correct type, expiry,
  groups, description) → second poll token-free → Secret deleted, zero left.
- Release pipeline: 5/5 builds + publish green; fresh download checksum OK;
  release binary `--plain` correct on overrides and real hardware.
- `test-join.sh` 4/4 throughout; `gofmt`/`vet`/`diff --check` clean at
  every commit; secret scans clean (only fake/test values).

## 5. Live cluster snapshot (2026-09-11)

Control panel, Headlamp, and Bob's demo session all 1/1 Running;
per-session Service (7682/8080/9002) and bound 5Gi PVC; Traefik ingress on
4 hostnames; LAN port-forwards `:8082` (API) and `:8080` (Headlamp);
Headlamp viewed from the guest laptop with a 24h token.

## 6. Key decisions log

- Go control panel over NestJS (single static binary; frontend stays JS).
- Per-session Deployment/Service/PVC over fixed StatefulSet + shared Service.
- Cookie + host-subdomain gateway over path routing (legacy removed, R1).
- In-cluster Deployment over root systemd unit (report amended; tokens via
  bootstrap Secrets, watcher via client-go).
- GPU requests conditional (`0` = omit) instead of faking node capacity.
- Demo stand-in image for plumbing proof, explicitly not a workspace.
- Bubble Tea TUI over Fyne/Electron (zero budget, no signing); darwin ships
  guest-only; no silent installs ever (D4); no Guest overrides (D1).
- Plain Actions releases over GoReleaser; `MASTER_IP` required, never defaulted.

## 7. Known limitations (all documented, none hidden)

- Session + node stores are in-memory/emptyDir: restarts orphan K3s
  resources (observed twice; R2).
- Workspace is the 82MB stand-in: editor 200s prove plumbing only (R6).
- No student frontend UI: login/launch/workspace pages don't exist (R4);
  API + curl work, browsers get 401/404 without a session+cookie.
- No TLS on the panel API (LAN trust assumed); no CSRF tokens yet (R3).
- `K3S_JOIN_URL` unset: approved polls mint then 503 (by design until a
  real master exists).
- No real K3s master or agent join ever attempted (k3d can't take real
  agents); Windows/WSL2 and P2 LAN-mDNS paths unverified on hardware.
- Legacy `join-cluster.sh` still the only working join path end to end.

## 8. Remaining work (ordered)

1. **Live second-laptop TUI + register rehearsal** (guest online; needs
   paired session): acceptance script `acceptance-second-laptop.sh` ready.
2. **Real K3s master + full agent join** (needs `K3S_JOIN_URL`, hardware,
   sudo both ends) — the single biggest unverified path.
3. **R3 CSRF**, **R2 persistence** (subsume nodes JSON too).
4. **R4 student frontend** (design complete in `uiux-design.md`).
5. **P2 LAN-mDNS proof** from the guest; **P5** Windows/WSL2; **P6**
   deprecation + `plan.md` update.
6. **R6 real Jazzy/Harmonic image** on GPU hardware; **R7** TLS, quotas,
   network policy, observability.

## 9. Reproduce / operate

`docs/local-cluster-headlamp.md` (cluster up/down, deploys, port-forwards,
tokens, troubleshooting), `docs/roadmap.md` (map), release downloads at
`github.com/GITGUYX8/LDNDRC/releases` (`v0.2.0` current), second-laptop
entry point `acceptance-second-laptop.sh` (requires `MASTER_IP`, currently
`192.168.1.37` — DHCP moves it; never hardcode).
