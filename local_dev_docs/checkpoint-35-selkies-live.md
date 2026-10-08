# Checkpoint 35 — Selkies 2.0.0 Live: Desktop 200 (detailed)

## Context

R6 Selkies chunk finale on `feat/r6-workspace-image`: new image tag
imported live, panel flipped, desktop stream proven end to end. No
repo code changes besides the tag bump + this report.

## Import + flip (observed)

- Operator `sudo k3s ctr images import
  ~/ldndrc-workspace-jazzy-harmonic-4aa29d15.tar`: index
  `4aa29d15…`, 65s unpack.
- Live `SESSION_IMAGE` patched `eaaf8088` → `4aa29d15` via
  `kubectl set env` (surgical — this branch's manifest is main-era);
  committed manifest + README bumped to the same tag.
- Delete-old-first panel restart → new pod Running (in-memory
  sessions wiped, known R2 debt).

## Live matrix GREEN (session `9007211b3f88`, Selkies image)

| Check | Observed |
|---|---|
| `POST /api/sessions` | 202 `provisioning` → `ready` (3rd 10s poll, no pull) |
| `desktop` + cookie | **200 Selkies HTML client** — the 502 era is over |
| `editor` + cookie | 302 → IDE (regression holds) |
| `gazebo` + cookie | 404 from live bridge (regression holds) |
| `desktop` no cookie | 401 |
| `DELETE` session | `stopped` |

## Dirty-tree discipline

Only manifest tag bump + README + this report (staged for commit).
The 8 join/nodes files + `join-cluster.sh` untouched/unstaged;
untracked junk left alone.

## What remains

Selkies migration done (CP-34…CP-35). Queued: merge with
`feat/r3-csrf` (7 commits), R2-final persistence, join-time image
pre-warm / registry mirror, GPU follow-up (`SESSION_GPU_LIMIT=1` +
device plugin).
