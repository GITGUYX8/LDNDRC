# Checkpoint 33 — R6 Live Matrix with Real Image (detailed)

## Context

Sixth R6 chunk on `feat/r6-workspace-image`: operator imported the
2.3GB tarball (`ctr images import`, index `cc54d71348fb`, 164s
unpack). This run flips the live panel to the real image and proves
a full session on it. No repo changes this chunk except this
report (the CP-32 manifest flip was already committed).

## Rollout (observed)

- Live `SESSION_IMAGE` was still `ldndrc/demo-standin:dev` →
  patched to `ldndrc/ros2-gz:jazzy-harmonic-eaaf8088` via
  `kubectl set env` (surgical: this branch's manifest is main-era
  and would clobber live-only env on wholesale apply).
- Delete-old-first panel restart (hostNetwork `:8082` discipline) →
  new pod `…-q4xpg` Running. In-memory session records wiped as
  known R2 debt — the 4 old stand-in workloads orphaned again.

## Live matrix GREEN (session `713105823791`, real image)

| Check | Observed |
|---|---|
| `POST /api/sessions` | 202 `provisioning` → `ready` in ~20s (no pull — pre-imported; pod `1/1 Running`) |
| `editor` + cookie | **302 → 200 VS Code IDE HTML** through the gateway (first real editor on this master) |
| `desktop` + cookie | 502 by design (Selkies deferred) |
| `gazebo` + cookie | 404 from the live bridge (routing proven, matches CP-29 semantics) |
| `editor` no cookie | 401 |
| `DELETE` session | `stopped`, workload terminating |

## Dirty-tree discipline

No source files touched; only this report (staged for commit). The
8 join/nodes files + `join-cluster.sh` untouched/unstaged;
untracked junk left alone.

## What remains

R6 core is done: build → verify → import → flip → live matrix, one
checkpoint per chunk (CP-27…CP-33). Queued: Selkies 2.0.0 migration
(`WITH_SELKIES=1`), image pre-warm / registry mirror in the join
flow, R2-final persistence, merge with `feat/r3-csrf` (7 commits).
