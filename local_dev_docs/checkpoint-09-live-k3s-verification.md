# Checkpoint 9 — Live K3s Verification of Sessions and Gateway

## The big picture first

Until now every claim about Kubernetes was backed by unit tests with a fake
client. This checkpoint put the real thing underneath: a live K3s cluster
(via k3d) on this machine, the actual control-panel container, and real HTTP
requests exercising login, provisioning, ownership, teardown, and gateway
enforcement. The cluster behaved exactly as the manifests and code promised —
including refusing to schedule work where it should not run.

Think of it as the first rehearsal with a real stage instead of a scale
model. The actors (sessions, gateway, RBAC) all hit their marks; the missing
set piece is the workspace image itself, which is too heavy to build here.

## What got built

| Piece | Friendly name | What it does |
|---|---|---|
| k3d cluster `ldndrc` | The Rehearsal Stage | Single-node K3s v1.35.5 cluster with Traefik ports 80/443 published. |
| `ldndrc/control-panel:dev` image | The Touring Company | Control-panel binary built by Docker and imported into the cluster. |
| Live namespace + RBAC + Secret | The Locked Dressing Rooms | Real `ldndrc` namespace, dedicated ServiceAccount, scoped Role, JWT Secret. |
| `control-panel-ingress.yaml` applied | The New Signposts | Four hostnames routed through Traefik to the control panel. |
| This report | The Review | What was proven live, with exact observed outputs. |

No application code changed in this checkpoint. The working tree still holds
the checkpoint-08 changes (gofmt, `go mod tidy`, README fixes) uncommitted.

## How each piece works

### The Rehearsal Stage — k3d cluster

k3d 5.9.0 (installed user-local via mise, no sudo) created a one-server K3s
cluster with the loadbalancer publishing 80/443 to the laptop. `kubectl`
1.37.0 (also via mise) talks to it. Docker became usable after the user ran
the sudo commands (group membership + daemon start).

### The Touring Company — image build and import

`docker build -t ldndrc/control-panel:dev ./control-panel` succeeded and
`k3d image import` placed it on the cluster node, so the Deployment's
`IfNotPresent` pull never needed a registry.

### The Locked Dressing Rooms — RBAC proof

`auth can-i` checks against the control-panel ServiceAccount returned `yes`
for creating Deployments and deleting PVCs in `ldndrc`, and `no` for
cluster-scoped node reads. The provisioner has exactly the keys it needs.

### The Signposts — ingress proof

With the ingress applied, requests to port 80 with `Host:
editor.ros-platform.local` reach the gateway: valid cookie on a provisioning
session → 503; no cookie → 401. The `control` hostname reaches the API
(404 on `/` is expected — no frontend route exists yet).

## How it all connects

1. Browser logs in → control panel signs JWT, sets `ldndrc_session` cookie.
2. `POST /api/sessions` → real Deployment + Service + PVC appear in K3s.
3. Scheduler evaluates Host label and GPU request before placing pods.
4. Gateway resolves the user's ready session to its private Service.
5. `DELETE /api/sessions/{id}` → Deployment, Service, and PVC disappear.

## Proof it works

Observed live on the cluster:

- `rollout status deployment/ldndrc-control-panel` → successfully rolled out,
  pod 1/1 Running.
- `curl :8082/healthz` → `{"status":"ok"}`.
- Alice session create → Deployment `ros2-session-1c1c7298267a`, Service with
  ports 7682/8080/9002, PVC `-home` — all present in `kubectl get`.
- Bob session create → second Deployment/Service/PVC with `rosDomainId` 101
  (Alice had 100). Alice's `GET /api/sessions` lists only her session.
- Fresh cluster, no `role=host` label → pods Pending with
  `didn't match Pod's node affinity/selector`. Host scheduling enforced.
- After labeling the node `role=host` → scheduler advances to
  `Insufficient nvidia.com/gpu`. GPU request enforced (k3d node has no GPU).
- Gateway via port-forward with Alice cookie on editor host → 503
  (provisioning, not ready). No cookie → 401. Other hosts → API as usual.
- Gateway via Traefik `:80` with Bob cookie → 503; no cookie → 401.
- Alice session delete → status `stopped`, and her Deployment, Service, and
  PVC are gone from the cluster; Bob's resources untouched.
- `test-join.sh` → 4 passed, 0 failed.
- `go test ./...`, `go vet`, `go build`, `gofmt` → all green (checkpoint 8).

## The one-liner version

| Piece | One-liner |
|---|---|
| Live cluster | Real K3s runs the control panel and provisions per-user resources. |
| RBAC | Scoped exactly right: sessions yes, cluster reads no. |
| Scheduler | Refuses non-Host nodes and GPU-less nodes, as designed. |
| Gateway | Live 503/401 behavior matches the unit tests. |
| Teardown | Deleting a session removes all three of its K3s resources. |

## What's next

- [ ] Build the Jazzy/Harmonic workspace image (heavy; needs disk + time) and
      import it so a session can reach `ready` end to end.
- [ ] Re-run the two-user test to green `ready`, then confirm editor/desktop
      proxying into live workspace ports.
- [ ] Add `/etc/hosts` or LAN DNS entries for the four `.ros-platform.local`
      names (needs sudo) instead of Host-header testing.
- [ ] Remove legacy `ros2-ingress.yaml` path routes after full verification.
- [ ] Commit the checkpoint-08 working-tree changes (gofmt, go.sum, README).
- [ ] Next feature chunk: CSRF protection on state-changing browser requests,
      then the student frontend against the verified API.
