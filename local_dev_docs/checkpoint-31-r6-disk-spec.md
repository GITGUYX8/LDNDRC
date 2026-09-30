# Checkpoint 31 — R6 Disk-Spec Budget (detailed)

## Context

Small R6 chunk on `feat/r6-workspace-image`: the join flow's 25GB
free-space gate predates the 10.2GB workspace image and its math
still described the stand-in era. Per operator decision: document
the real budget, keep the threshold. No behavior change, no
threshold change.

## Changes (2 files)

- `control-panel/internal/join/hardware.go`: budget comment on
  `DefaultThresholds` — 10.2GB image + ~1GB agent + 5Gi
  first-session PVC + ~2GB headroom ≈ 18–20GB, so 25GB holds with
  margin. Revisit triggers noted: 3+ local sessions (15Gi PVCs) or
  image past ~12GB.
- `docs/host-joiner-tui-spec.md`: Disk row green-cell updated from
  the stale "(join ~1.5 GB + first workload pull ~5–10 GB +
  headroom)" to the R6 budget. Spec and code agree again.

## Proofs

- `gofmt -l` clean, `go test ./internal/join/` ok (10.2s).
- Threshold value untouched (`MinDiskGB: 25`); detection
  (`diskFreeGB("/")`) untouched.

## Dirty-tree discipline

Only the 2 spec files above + this report (staged for commit). The
8 join/nodes behavior files + `join-cluster.sh` untouched/unstaged;
untracked junk left alone.

## Next

Live matrix chunk: operator `sudo k3s ctr images import`, then
`SESSION_IMAGE` flip (committed) → session → ready with real
services → checkpoint. Still queued: Selkies 2.0.0 migration,
registry-mirror / join-time pre-warm step, R2-final.
