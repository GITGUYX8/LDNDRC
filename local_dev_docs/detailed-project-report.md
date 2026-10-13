# LDNDRC — Detailed Project Report (Technical Appendix)

**Date:** 2026-10-13 · **Branch:** `main@299702f` · **Cluster:** real K3s organizer `hari` (K3s `v1.36.4+k3s1`, `192.168.1.37:6443`)
**Merges:** `d9d9a28` (`feat/r3-csrf`: R2/R3/R4) + `e5fe6cb` (`feat/r6-workspace-image`: CP-27…CP-38) · **Releases:** `v0.1.0`, `v0.2.0` (`ldndrc-join`)
**Session picture:** `ldndrc/ros2-gz:jazzy-harmonic-10f1f10c` (10.8GB, Selkies 2.0.0, Fuel-seeded)

> How to read this doc (new contributor): this is the *appendix*. The friendly
> story lives in `docs/project-journey-report.md` (final-vs-now, mind-map,
> brackets, decision graph); the build order lives in `docs/roadmap.md`.
> Here each section starts with 2 plain-English lines (italic), then the
> precise inventory. Checkpoints `local_dev_docs/checkpoint-*.md` (39 stories)
> hold the raw proofs. Terms in (brackets) match the journey doc's glossary.

---

## 1. Project identity

*Plain words: a classroom of laptops becomes a robot school with no cloud.
Strong laptops (Hosts) share power and run sealed robot boxes; weak laptops
(Guests) just use a browser; one laptop (Master) organizes.*

LDNDRC (Local Dynamic Node Discovery ROS Cluster) is a local-first,
browser-accessible robotics simulation platform: consumer laptops form a K3s
cluster (K3s: lightweight organizer deciding what runs where), strong laptops
("Hosts") run isolated per-user ROS 2 workspaces (ROS 2: robot brain;
workspace: sealed box with editor + sim + desktop), weak laptops ("Guests")
use a browser. No cloud, no dedicated servers.

## 2. Architecture as built

*Plain words: your laptop talks to the front door, the desk checks your
wristband and serves the website, the organizer starts your sealed box,
and you can peek through the glass wall.*

```text
Guest browser / joining laptop (same Wi-Fi, nicknames in hosts file)
  │  editor|desktop|gazebo|control .ros-platform.local → Traefik :80/:443 (front door)
  ▼
Control panel / desk (Go program, runs on the organizer's own network, modest container)
  ├─ /api/auth/*          login: JWT pass (signed proof) + wristband cookie + click stamp (R3)
  ├─ /api/sessions/*      boxes: create/list/stop, yours only, written in the notebook (R2)
  ├─ /api/nodes/*         welcoming: ask/wait/approve/deny/list/health-report
  ├─ /                   website inside the desk file (R4: Login/Launch/Workspace/Desktop/Admin)
  └─ gateway (guard)      checks wristband, forwards to your box's doors (FQDN dot: full address with trailing dot)
K3s API (organizer's controls, driven with client-go: the Go remote): per-box
  Deployment (recipe) + Service (address, doors 7682/8080/9002) + home PVC (5GB private disk);
  join tickets as Secrets (15-min passwords); laptop labels; health checks; restart re-read
Headlamp (glass wall, token key): operator sees everything
mDNS (Wi-Fi shout "_ldndrc-master._tcp"): organizer announces itself on hari:8082
```

Key facts, precisely: one Deployment + Service + PVC per box with a unique
`ROS_DOMAIN_ID` (channel number so boxes can't hear each other) and
`ROS_AUTOMATIC_DISCOVERY_RANGE=LOCALHOST` (robots talk inside their own box
only); Host-only placing via `node-role.kubernetes.io/role` (label: this
laptop shares power); graphics optional (`SESSION_GPU_LIMIT=0` on plain-CPU
today, `"1"` on NVIDIA hosts); browsers use wristband cookie + `X-CSRF-Token`
stamp on changes, scripts use Bearer pass; box addresses dialed as FQDN
(trailing dot — skips the Wi-Fi's wildcard-DNS trap found in CP20);
organizer updates use delete-old-first (door `:8082` fits one copy);
picture tarballs imported from `$HOME` (2.4GB packed; `/tmp` too small/wiped).

## 3. Work inventory

### 3.1 Desk backend (`control-panel/`, Go 1.26.5)

*Plain words: the reception desk program — who you are, what boxes exist,
who may join, which door leads where, and how new organizers are found.*

| Part | Job (precise) | State |
|---|---|---|
| `cmd/server` | Startup wiring: passes, notebooks, ticket-maker, watcher, guard; `ADVERTISE_MDNS` switch; restart re-read | Done, live |
| `internal/auth` | Sign/verify passes (JWT), set/check/clear wristband cookie, make/check click stamps (`IssueCSRF`/`VerifyCSRF`: HMAC fingerprint) | Done, live |
| `internal/httpapi` | Box CRUD + laptop-welcoming API + health-report upload (1MB cap, ask-door speed-limited, `JOIN_KEY`, `AUTO_APPROVE`) + stamp guard (`guardCSRF` → 403) + website catch-all (`spa.go`) | Done, live (login-skip fix next) |
| `internal/sessions` | Notebook store (`SESSIONS_DB`, default `/var/lib/ldndrc/sessions.json`) + organizer driver (Deployment/Service/PVC, robot env, GPU-if-asked, health probes, safe user IDs); `Get()` re-checks starting→ready | Done, live |
| `internal/nodes` | Laptop-ask notebook (JSON file, crash-safe writes, repeat-asks collapse, approve/deny/expire rules) | Done, live (real disk = R2-final) |
| `internal/nodes` tickets | Join tickets as bootstrap Secrets via organizer API, `K10…` secure shape from cluster keys, given-once rule | Done, live |
| `internal/nodes` watcher | Every 5s: label Ready laptops as power-sharers (`role=host`), mark joined | Done, live |
| `internal/gateway` | Name-based forwarding (editor 7682 / desktop 8080 / sim-data 9002), full-address dialing, check-before-forward, honest codes (401/404/503/502) | Done, live |
| `internal/discovery` | Wi-Fi shout `_ldndrc-master._tcp` + info card | Done, shouting live on real organizer |
| `internal/join` | Pocket-tool engine: muscle thresholds (25GB free covers 10.8GB picture + 1GB agent + 5GB first disk + air), per-system checks (`TEST_*` fakes), shout-listen + typed-address fallback, ask/wait client, password-gated install, health-report pack; rehearsal fixes (`4eeffea`) | Done, tested |
| `cmd/join` + TUI | Text screens (Checks/Fix/Confirm/Join/Done, Bubble Tea library), `--plain` byte-identical mode, one-shot engine glue | Done, builds for 3 systems |

### 3.2 Student website (`control-panel/web/`, Vite + React 18 + TS)

*Plain words: the classroom site students and teachers click — enter, big
launch button, code + sim side by side, full-screen desktop, teacher cards.*

| Page | Job | State |
|---|---|---|
| `src/api.ts` + `auth.tsx` | Talk helper (pass + stamp + wristband on every call); passes kept in memory, mirrored for reloads | Done, live |
| `pages/Login.tsx` | Entry card, checks typing, show/hide secret, no self-signup (teachers create accounts) | Done, live |
| `pages/Launch.tsx` | No-box → Starting stepper (asks "ready?" every 2s via self-healing read; Cancel = delete) → Ready cards → Error panel with details + Retry | Done, live |
| `pages/Workspace.tsx` + TopBar/Rail/SplitStage | Code/Sim/Split frames (kept loaded, hidden not killed, so no reconnects), top bar, icon rail, draggable middle (position remembered) | Done, live |
| `pages/Desktop.tsx` | Screen-sharing frame + Back button + one-time keys hint | Done, live (picture 200) |
| `pages/Admin.tsx` | `/admin`: fleet counts, person/box/laptop cards, status filters, approve/deny | Done, live |
| Build | `node:22-bookworm-slim` build step in `Dockerfile`; `web/dist/` output re-created, sources committed | Done, live (47 parts, ~197KB/62KB packed) |

Sim frame shows the sim-data door today; the pretty 3D piece from
`sim-web-app/` (gzweb: browser-drawn 3D) is the viewer slice.

### 3.3 Robot-world picture (`images/`)

*Plain words: the frozen 10.8GB hard drive every sealed box boots from —
robot brain, 3D physics, editor, screen sharing, practice models included.*

| Layer | Contents | State |
|---|---|---|
| `images/base` | Ubuntu + Jazzy brain (desktop, navigation, robot↔3D bridge, TurtleBot3 drivers), chatter locked to localhost, student user 1000 | Built 8.25GB, done |
| `images/workspace` | Browser editor + 3D data pipe + screen manager + home starter (+ Fuel seed 64K: Ground Plane + Sun practice models) + Selkies 2.0.0 (`WITH_SELKIES=1` switch) | Built 10.8GB, live (`:jazzy-harmonic-10f1f10c`) |
| Tags | Frozen `:jazzy-harmonic-<code>` (`eaaf8088` → `4aa29d15` → `10f1f10c`), never moving `:latest` | Done; 2.4GB pack in `$HOME` |

History kept: Selkies 1.6.2 files vanished upstream (404) → shipped without
desktop, then moved to pure-Python 2.0.0 (no video-toolkit, sound-off is a
native switch). Packing math (CP38): ~4.5x squeeze, import 164s cold → 28s
warm (shared layers), Wi-Fi 1–4 min + unpack 1–3 min ≈ 5–10 min per new
laptop once, then kept; shelf-mirror worth it at ~5 laptops.

### 3.4 Recipes (`manifests/`)

*Plain words: the cluster's paperwork — who may touch what, the desk's
settings, the four web names, the teacher's glass wall.*

Least-power permissions (RBAC: boxes in `ldndrc`, tickets in `kube-system`,
laptop reads cluster-wide), desk Deployment/Service/Ingress (4 names,
`SESSION_IMAGE` = `jazzy-harmonic-10f1f10c`, `SESSIONS_DB` + `NODES_DB`
settings), secret example (never real keys), Headlamp set, organizer-network
patch for real masters (apply with `kubectl patch`, never `apply` the
fragment — it is a snippet, not a full paper). Known stale line: Selkies
"waiting/502" note predates the 200-live proof (tidy with login fix).
Practice-cluster override (tiny demo picture) kept.

### 3.5 Pocket-tool delivery

*Plain words: how the join program reaches laptops — automatic builds for
every system when we tag a version.*

`.github/workflows/release-join.yml`: on version tag, builds
(linux/windows/mac × 64-bit/ARM), runs checks first, publishes checksums
(`SHA256SUMS`: fingerprints to verify downloads) + notes. `v0.1.0` (Join
screen not wired) replaced by `v0.2.0` (current).

### 3.6 Docs map

*Plain words: where to read what — story, order, appendix, handbook.*

`plan.md` (original dream), `docs/project-journey-report.md` (friendly story:
final-vs-now, mind-map, brackets, decisions), `docs/roadmap.md` (build
order), `docs/local-cluster-headlamp.md` (handbook: start/stop, tokens,
fixes), `docs/host-joiner-tui-spec.md` (pocket-tool rules D1–D4 + disk
math), `docs/uiux-design.md` (site design + 3 as-built deltas),
`onboarding-design-report.md` (welcoming design, amended for in-cluster
reality), `control-panel/README.md` (deploy + welcoming API), 39 checkpoints.

## 4. Verification ledger (claim → proof → what we saw)

### Automatic tests (all `ok`: auth, discovery, gateway, httpapi, join, nodes, sessions)

*Plain words: the robot checks the computer runs on itself — no cluster needed.*

Ownership walls, no double-boxes, life-stage steps, repeat-asks collapse,
approve/deny/expire/re-approve, ticket-given-once, watcher
Ready/sleepy/gone cases, guard 401/503 paths, muscle gates ×9, graphics
bands, `--plain` word-for-word strings, screen keypress steps, report
round-trip + size cap, shout-then-hear round-trip, **stamp 7 guard + 5
binding cases, notebook save/load + corrupt-refuses + adopt/mark-stopped,
`Get()` flip/stay, website-fallback order.**

### Live on the real organizer (seen, not faked)

*Plain words: a human watched it happen on the real laptop `hari`.*

- Move-in (CP20): permissions + recovered login secret + organizer-network +
  Wi-Fi shout + front door on 4 names (`192.168.1.37`); box
  starting→ready; editor OK first time here; first shout heard. Two bugs
  found only here: full-address dot (wildcard-DNS trap), delete-old-first
  (update deadlock).
- Safety+memory (CP21): stamp matrix (no stamp → blocked, wrong → blocked,
  script pass → through) + editor still OK; killed container kept the same
  box ID, starting→ready on first visit, zero lost.
- Website (CP24/26): pages 200 with the real bundle; login gives
  pass+stamp+wristband; Launch waiting → ready (3rd check); editor-OK /
  desktop-then-200 / 401 / 403 all hold; `/admin` 200 with cards;
  approve/deny round-trip on live rows (`verify-laptop-a/b`); delete →
  `stopped` feeds the filter chips.
- Real world (CP33/35/37): ready ~20s (no download); editor redirect→IDE;
  desktop 200 screen page (502 era over); sim-data door answers with data;
  `turtlebot3-sim empty` in-box with zero download errors, full message bus,
  clock at full speed on plain CPU; seed present in home.
- Merge (CP39): format/vet clean, 7 packs ok, 47-part bundle identical,
  recipe carries new picture + notebook setting.
- Carried over: two-student walls + 403s, scheduler muscle/GPU refusals,
  full cleanup on delete, ticket-once over Wi-Fi, `--plain` 4/4, screen
  `q`→clean exit, permission probes, release checksum installs,
  `test-join.sh` 4/4, secret scans clean (fakes only).

## 5. Live snapshot (2026-10-09)

*Plain words: what you would see if you walked up to the organizer today.*

Real organizer `hari` (`192.168.1.37:6443`): desk (organizer-network, Wi-Fi
shout on), glass wall, per-box address (7682/8080/9002) and bound 5GB disks;
front door on 4 names; picture `jazzy-harmonic-10f1f10c`; door codes
editor-IDE / desktop-200 / sim-data / 401-no-wristband / 403-no-stamp;
`/admin` live with cards; shout live; firewall 6443/80/443/5353/8472.
Practice cluster `ldndrc` parked (rollback kept). Leftovers: 2 clearly-named
test laptop rows (no delete-button exists yet), pre-check workloads orphaned
by delete-old-first restarts (full-memory step ends this).

## 6. Key choices log (short: chosen → dropped)

*Plain words: the branch-by-branch decisions — details live in journey §5.*

Go desk → not Node · per-student boxes → not shared room · cookie+names →
not old paths · desk inside cluster → not systemd · 15-min Secrets → not
manual tokens · GPU-optional → not faked capacity · tiny picture first →
not 10GB day one · text screens → not heavy apps · HMAC stamps → not
nothing · notebook+adopt/mark-stopped → not silent delete · site inside Go
file → not sidecar · cards Admin → not plain table · Selkies 2.0 behind
flag → not dead 1.6.2 · pre-pack 2 models → not download at start · frozen
tags + `$HOME` pack → not moving tag + `/tmp` · two tracks, one merge.

## 7. Known limits (nothing hidden)

- **Login small bug** (arrived CP39): old-wristband login attempt → 403.
  Next: skip stamp check on login.
- Small restarts safe; whole-copy redeploys still lose track (notebooks on
  temporary shelf — full-memory step moves them to disk; graceful-update
  flag still default).
- Recipe comment stale ("Selkies waiting/502", really 200-live).
- No pretty sim viewer yet (data frame today).
- No guest full-join yet (practice cluster couldn't take real agents; real
  organizer waits for the paired run). Guest hearing the shout pending.
- No encryption yet (trusted Wi-Fi assumed); click-stamps are the only
  browser armor so far (R7: encryption, walls, limits, graphs).
- Windows/WSL2 unproven on hardware; shelf-mirror/pre-load measured (CP38)
  not built; slimming/speed-packing parked; light picture parked.
- Old bash join path still the only complete end-to-end road.

## 8. Remaining work (ordered — same as roadmap §2/§3)

1. **Login fix** (< 0.5 day): skip login in stamp guard + test + rebuild +
   delete-old-first rollout + live proof; tidy stale Selkies line.
2. **Full memory** (0.5–2 days): notebooks → disk/DB; graceful desk updates;
   prove redeploy loses nothing.
3. **Pretty viewer** (0.5 day): gzweb piece into the site; `world`/`house`
   smoke. Optional: see-everyone Admin feed; glass-wall teacher lock.
4. **Two-laptop join** (days, second laptop): full acceptance run — ask →
   approve → ticket → real install → marked-joined → box runs there; no-path;
   shout hearing.
5. **Windows proof + docs sunset** (days).
6. **Sturdy** (ongoing): encryption, limits, walls, graphs, logs.
7. **Picture parallel:** shelf-mirror/pre-load at ~5 laptops; graphics
   follow-up (`SESSION_GPU_LIMIT=1` + device plugin).

## 9. Run / operate

*Plain words: how to start it and where to click.*

Handbook `docs/local-cluster-headlamp.md` (start/stop, deploys,
port-forwards, keys, fixes), story `docs/project-journey-report.md`, order
`docs/roadmap.md`, downloads `github.com/GITGUYX8/LDNDRC/releases`
(`v0.2.0` current), two-laptop script `acceptance-second-laptop.sh` (needs
`MASTER_IP`, today `192.168.1.37` — Wi-Fi addresses move, never hardcode).
Organizer rule: never plain-restart the desk — delete the old copy first.
