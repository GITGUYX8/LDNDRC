# Checkpoint 13 — Legacy Ingress Removal

## The big picture first

Since checkpoint 7 the cluster has carried two front doors: the new
host-based ingress (`control`, `editor`, `desktop`, `gazebo` subdomains)
and the legacy path-based one (`/code`, `/stream`, `/sim` on the bare
hostname, backed by a shared Service and a fixed 3-replica StatefulSet).
The legacy door was kept purely as rollback insurance. With the host routes
proven live across checkpoints 9 and 12, the insurance expired — and dead
routes are a footgun (a future deploy could resurrect the shared-service
model by accident). This checkpoint removed them.

Think of it as taking down the old wooden sign after the new neon one has
run glitch-free for weeks.

## What got built

Nothing new — this checkpoint is a deletion:

| Removed | Why it was safe |
|---|---|
| `manifests/ros2-ingress.yaml` | Never applied live; host ingress verified instead |
| `manifests/ros2-service.yaml` | Only referenced by the deleted ingress |
| `manifests/ros2-platform.yaml` | Fixed StatefulSet superseded by per-session Deployments |
| Stale "keep until verified" comments | Verification happened (CP09/CP12) |

Kept deliberately: `test-pod/svc/ingress.yaml` (separate checkpoint-01
scaffolding, never applied live — its own cleanup later) and the `plan.md`
design narrative (historical record of how the architecture evolved).

## How it was verified

- Repo-wide search: no Go file, script, or live manifest references
  `ros2-platform-svc` or the old paths. Only docs mention them.
- Live cluster check: `kubectl get ingress -A` shows solely
  `ldndrc-control-panel`; no StatefulSet or shared Service ever existed
  in the `ldndrc` namespace.
- All 9 remaining manifests still YAML-parse.
- Gateway re-verified after removal: editor 200, desktop/gazebo 502
  (stand-in, by design), no-cookie 401, health 200.
- The old `/code` path on the bare hostname now returns 404 — proof no
  route (and therefore no user flow) depended on it.
- `git diff --check` clean.

## The one-liner version

| Piece | One-liner |
|---|---|
| Three deleted manifests | The old shared-service front door is gone. |
| Gateway re-check | Host routes behave exactly as before the removal. |
| Bare-host `/code` → 404 | Confirms nothing depended on the legacy paths. |

## What's next

- [ ] R3 CSRF protection on state-changing browser requests.
- [ ] R2 session persistence (restarts still orphan K3s resources).
- [ ] R4 student frontend UI against the verified API.
- [ ] Commit this checkpoint's changes when ready.
