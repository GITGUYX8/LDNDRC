# Checkpoint 14 — Nodes API: Register, Approve, Mint (P1)

## The big picture first

A second laptop cannot join the cluster today without a human pasting the
all-powerful static K3s token. This checkpoint built the first half of the
zero-touch flow from the amended onboarding report: laptops register
themselves, an operator approves with one API call, and the laptop's poll
receives a single-use 15-minute bootstrap token minted through the
Kubernetes API — no root, no CLI exec, no static secret changing hands.

Think of it as a coat check: the laptop hands over a claim ticket
(registration), the operator nods (approval), and the desk hands back
exactly one key that melts in 15 minutes.

## What got built

| File | Friendly name | What it does |
|---|---|---|
| `internal/nodes/store.go` | The Coat-Check Ledger | JSON-file store (atomic writes): idempotent register, approve/deny/expire transitions, token IDs without token secrets. |
| `internal/nodes/tokens.go` | The Key Cutter | Mints bootstrap-token Secrets in `kube-system` via client-go, formats `K10…` secure tokens from the cluster CA. |
| `internal/nodes/watcher.go` | The Usher | Every 5s, labels Ready nodes `role=host` and marks them joined. |
| `internal/httpapi/router.go` | The Front Desk | Five routes: public register (rate-limited, optional join key) + poll, JWT-gated list/approve/deny. |
| `cmd/server/main.go` | The Wiring | Loads the nodes DB, builds the minter + watcher in-cluster, serves the new routes. |
| `manifests/control-panel-rbac.yaml` | The Key Ring, Extended | kube-system secrets role + nodes ClusterRole for the control-panel account. |
| `manifests/control-panel-deployment.yaml` | The Filing Cabinet | `emptyDir` volume for the nodes DB plus `NODES_DB`/`K3S_JOIN_URL` env. |
| `control-panel/README.md` | The Instructions | Documents the register → approve → poll flow. |

## How it was verified

Unit level (`go test ./...` all `ok`):

- Register idempotency, bad-input rejection, approve/deny/expiry/re-approve
  transitions, JSON reload round-trip.
- Minter output shape (`K10…::…`), secret fields (groups, description),
  CA-missing failure.
- Watcher joins Ready nodes with both labels; skips unready and missing ones.
- HTTP flow: register 201 → anon list 401 → approve 200 → poll returns the
  token exactly once → second poll returns status only; deny path; unknown
  ID 404.

Live on the k3d cluster:

- RBAC probes: secrets create in `kube-system` yes, nodes patch yes.
- Real register → approve → poll minted bootstrap-token Secret
  `bootstrap-token-rng9cl` (correct type, expiry, groups); second poll did
  not re-mint; poll answered 503 "join URL not configured" because
  `K3S_JOIN_URL` is intentionally unset here (no joining laptop exists).
- Deny-after-approve correctly rejected per the state machine; test secret
  deleted afterwards.

## The one-liner version

| Piece | One-liner |
|---|---|
| `nodes/` package | Laptops check in, operators approve, keys melt in 15 minutes. |
| Router + main wiring | Five routes live behind the existing auth, watcher ticking. |
| RBAC + volume | Minter and watcher have exactly the permissions they need. |

## What's next

- [ ] P2: mDNS advertisement (hostNetwork vs sidecar decision) + master docs.
- [ ] P4: `ldndrc-join` Linux path end-to-end against this API.
- [ ] Set `K3S_JOIN_URL` when a real master with joining laptops exists.
- [ ] R2 persistence can subsume the nodes JSON store later.
- [ ] Commit this checkpoint's changes when ready.
