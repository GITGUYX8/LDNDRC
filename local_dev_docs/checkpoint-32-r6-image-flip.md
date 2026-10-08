# Checkpoint 32 — R6 SESSION_IMAGE Flip (detailed)

## Context

Fifth R6 chunk on `feat/r6-workspace-image`: declare the real image
as the session image in manifests + docs. No rebuild, no behavior
change yet — the live cluster still runs the stand-in until the
import + rollout chunk. GPU stays `"0"` (CPU-only decision).

## Changes (2 files)

- `manifests/control-panel-deployment.yaml`: `SESSION_IMAGE`
  `ldndrc/demo-standin:dev` →
  `ldndrc/ros2-gz:jazzy-harmonic-eaaf8088` (immutable sha tag, never
  `:latest`), with a comment recording the R6 rationale, the k3d
  demo override path, the Selkies-deferred 502, and the CPU-only GPU
  note.
- `README.md`: env table default updated to the immutable tag.
  Path A (k3d demo on the stand-in) is untouched and still valid.

## Proofs

- Python YAML parse OK; `kubectl apply --dry-run=client` accepts the
  manifest against the live cluster.

## Dirty-tree discipline

Only the 2 files above + this report (staged for commit). The 8
join/nodes files + `join-cluster.sh` untouched/unstaged; untracked
junk left alone.

## Next (blocked on operator sudo)

`sudo k3s ctr images import
~/ldndrc-workspace-jazzy-harmonic-eaaf8088.tar` → apply the flipped
manifest (or patch the live Deployment env) → new session → ready
with real services (editor 200 IDE, sim bridge, desktop 502) →
checkpoint. Still queued: Selkies 2.0.0 migration, image pre-warm /
registry mirror in the join flow, R2-final, merge with
`feat/r3-csrf`.
