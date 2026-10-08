# Checkpoint 26 — R4 UI-3 Live Verify on Real Master (detailed)

## Context

Continues on branch `feat/r3-csrf`, unpushed. Checkpoint-25 built
UI-3 (reference user/session cards) as local commit `fdd2dc1` and
queued the live rollout. Meanwhile the master node **rebooted**
(pods show restarts ~5h ago, `/tmp` tmpfs wiped including the saved
tarball). This run: re-saved the tarball from Docker's on-disk store,
imported, rolled out, verified. No code changes (image = `fdd2dc1`
source); dirty join/nodes files untouched.

## Step-by-step with observed outputs

### 1. Tarball recovery

- `sudo k3s ctr images import` failed: `/tmp` tarball gone with the
  reboot. Docker store survived: image `528c3359` (built 05:29 from
  the cards source) intact.
- `docker save` → `/tmp/ldndrc-control-panel-dev.tar` (14M) again.
- Operator import: index `sha256:528c3359…` confirmed.

### 2. Rollout (delete-old-first, never plain `rollout restart`)

- Deleted pod `…-qhshf` (old image `48724709…`, UI-1/UI-2) →
  `rollout status` success in seconds → new pod `…-7hf9d`
  `1/1 Running`, `hostNetwork=true`.
- Note: containerd reports `imageID=7d9f9634…` (config digest form,
  differs from the docker index digest) — confirmed as the new image
  by content, not by digest string (below).

### 3. Live verify GREEN (plain DNS — the `/etc/hosts` fix works)

| Check | Observed |
|---|---|
| `GET /admin` | 200, new bundle `index-lFfXNHjr.js` (cards code live) |
| Login (instructor) | token 160 + csrf 64 + cookie |
| `GET /api/nodes` no bearer | 401 (regression holds) |
| Cookie POST without `X-CSRF-Token` | 403 (regression holds) |
| `GET /api/nodes` fresh | `[]` (emptyDir wiped by reboot — expected) |
| Register `verify-laptop-a/b` | both `pending` (idempotent re-register dedupes) |
| Approve A / deny B + reason | `approved` / `denied` — the exact payloads the host cards render |
| `POST /api/sessions` | 202 `provisioning` → `ready` (3rd 10s poll) |
| `editor` + cookie | 200 (student path regression holds) |
| `editor` no cookie | 401 · `desktop` + cookie 502 by design |
| `DELETE /api/sessions/{id}` | `stopped` — feeds the session cards' `stopped` filter chip |

The stopped workload `ros2-session-fed39e360a7c` was `Terminating`
at read time (cleanup in flight).

## Residue to know about

- Live nodes DB now holds 2 verify rows (`verify-laptop-a`
  approved, `verify-laptop-b` denied "verify run, not a real
  laptop") — no node-delete API exists, so they persist as cards.
  Harmless and clearly named.
- Pre-existing orphaned session workloads from checkpoint-24 still
  Running (R2-final resolves for good).
- Live Deployment still predates `SESSIONS_DB` env and
  `maxUnavailable: 1` — queued with R2-final.

## Next

R2-final (nodes/sessions → PVC/DB) + panel Deployment
`maxUnavailable: 1`. Optional later: `GET /api/admin/sessions`
list-all seam (fills Users room-wide, no view change); ADMIN-role
gating for headlamp (checkpoint-10).
