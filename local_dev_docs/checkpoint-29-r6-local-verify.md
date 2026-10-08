# Checkpoint 29 — R6 Local Contract Verify (detailed)

## Context

Third R6 chunk on `feat/r6-workspace-image`: prove the 10.2GB
workspace image boots and satisfies the session contract from
`control-panel/internal/sessions/kubernetes.go` (ports 7682/8080/
9002, readiness `GET /` on 7682, uid 1000, `/home/student`) using a
plain `docker run` — no cluster involved. No image changes needed.

## Verify (observed, container `ws-verify`)

- `id` → `uid=1000(student) gid=1000(student)` ✅ (matches the pod
  `runAsUser: 1000`; entrypoint fell back to `ROS_DOMAIN_ID=30`
  with no `POD_NAME`, as designed for non-StatefulSet runs).
- `supervisorctl status` → `code-server RUNNING`, `gazebo-web
  RUNNING`, both past `startsecs` ✅ (no selkies section — deferred
  per checkpoint-28, no restart loop).
- `:7682/` → **302** `Location: ./?folder=/home/student/dev_ws` →
  **200** VS Code HTML ✅. The 302 is code-server's normal folder
  redirect under `auth: none`; kubelet treats ≤399 as success so the
  session readiness probe passes once code-server is up.
- `:9002/` → **404 from a live server** ✅ (port bound, bridge
  `RUNNING` via `gz launch` + `websocket.gzlaunch`; root simply isn't
  a route on the Harmonic-era bridge — guessed `/gazebo`, `/status`,
  `/gzweb` also 404, which only maps the route table, not a failure).
- `:8080` → **connection refused** ✅-by-design (Selkies deferred;
  gateway `desktop` will stay 502 exactly like the stand-in).

## Dirty-tree discipline

No repo files touched this chunk except this report (staged for
commit). The 8 join/nodes files + `join-cluster.sh` untouched/
unstaged; untracked junk left alone. Test container removed.

## Next chunk

Immutable tag (`:jazzy-harmonic-<sha>`) + `docker save` → operator
`sudo k3s ctr images import` → checkpoint-30. Then `SESSION_IMAGE`
flip (committed) → live session matrix with real services.
