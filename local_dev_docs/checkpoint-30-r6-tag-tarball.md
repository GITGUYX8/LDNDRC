# Checkpoint 30 — R6 Immutable Tag + Import Tarball (detailed)

## Context

Fourth R6 chunk on `feat/r6-workspace-image`: freeze the verified
workspace image under an immutable tag and stage it for cluster
import. No image rebuild, no code changes.

## Tag + tarball (observed)

- `docker tag ldndrc/ros2-gz:jazzy-harmonic-workspace
  ldndrc/ros2-gz:jazzy-harmonic-eaaf8088` — `<sha>` is the image
  config digest short form (`eaaf8088…`), per the workshop-design rule
  (`:jazzy-harmonic-<sha>`, never `:latest` for sessions).
- `docker save` → `~/ldndrc-workspace-jazzy-harmonic-eaaf8088.tar`
  (**2.3GB** on disk — layers compress well from 10.2GB).
- Location note: `/tmp` is a 7.7GB tmpfs and would not reliably hold
  this tarball (plus it wipes on reboot, as checkpoint-26 learned),
  so it lives in `$HOME` (416GB free). `k3s ctr images import` reads
  it via sudo regardless of path.

## Dirty-tree discipline

No repo files touched except this report (staged for commit). The 8
join/nodes files + `join-cluster.sh` untouched/unstaged; untracked
junk left alone.

## Next chunk

Operator runs `sudo k3s ctr images import
~/ldndrc-workspace-jazzy-harmonic-eaaf8088.tar`, then: flip
`SESSION_IMAGE` in manifests (committed) → live session matrix with
real services (editor 200 IDE, sim bridge, desktop 502-by-design) →
checkpoint.
