# Checkpoint 27 — R6 Base Image Build (detailed)

## Context

First chunk of the R6 real-workspace-image track on new branch
`feat/r6-workspace-image`, forked from `main` (`a404617`) per
operator direction — the 7 unpushed `feat/r3-csrf` commits stay
separate for a later merge. Scope this chunk: audit + build
`images/base` only. CPU-only (no nvidia toolkit/device plugin;
`SESSION_GPU_LIMIT` stays `"0"`). No code changes.

## Audit (pre-build, read-only)

- `images/base/Dockerfile` + `installs.sh`: `osrf/ros:jazzy-desktop-full-noble`
  + ROS Jazzy (desktop, nav2, ros-gz, TurtleBot3), dev toolchain,
  DDS pinned to `LOCALHOST`, `student` uid/gid 1000. Unpinned
  upstream tags kept as-is (genesis-tested recipe; digest-pinning is
  follow-up, not this branch).
- `images/workspace` scripts for CPU-only: `run-with-virtualgl.sh`
  already `exec`s directly when `nvidia-smi` is absent — zero changes
  needed for GPU-less pods. `init-student-home.sh` writes
  code-server `auth: none`, so the session readiness probe
  (`GET /` on 7682) gets a 200 once code-server is up.
- Build contexts are lean (wallpaper 540K, home-seed 60K) — no
  `.dockerignore` needed.
- Dirty-tree discipline: the 8 join/nodes modifications +
  `join-cluster.sh` (carried over from the old worktree) left
  untouched and unstaged.

## Build (observed)

`docker build -t ldndrc/ros2-gz:jazzy-harmonic-base ./images/base`
green: apt layer ~100s (incl. `uv` install), user setup <1s,
export ~50s.

Result: `ldndrc/ros2-gz:jazzy-harmonic-base` — **8.25GB**
(index `8ceac6d138f9`, config `64b2e462…`). Matches the roadmap's
~10GB+ expectation for the stack (workspace layer still to come).

## Next chunk

Build `images/workspace` on top of this base (code-server + Selkies
1.6.2 + gzweb under supervisord) → checkpoint-28. Then local
contract verify → import → `SESSION_IMAGE` flip → live matrix,
one checkpoint per chunk.
