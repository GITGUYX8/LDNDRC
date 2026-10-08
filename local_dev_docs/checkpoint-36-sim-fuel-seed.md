# Checkpoint 36 — Sim Smoke: Fuel Seeding + Offline Proof (detailed)

## Context

R6 sim chunk on `feat/r6-workspace-image`: launch a TurtleBot3
world and prove topics flow on CPU-only. First attempt failed for a
real, diagnosable reason; fixed by seeding; proven offline
(`--network none`). No Dockerfile change needed.

## Failure → diagnosis

- Live session `f6d4c988a688` ready; `turtlebot3-sim empty` in-pod
  died: `Unable to find uri[https://fuel.gazebosim.org/...Ground
  Plane]` + `[Sun]`, `REST response code: 0`, gazebo exited, launch
  tore down.
- Pod probe: `fuel.gazebosim.org` resolves to `185.38.109.x` parking
  IPs — the LAN wildcard-DNS trap (checkpoint-20 §5 class), now in
  session-pod DNS. Connection impossible, not just slow.
- Decision: **pre-seed models into the image** (offline-safe,
  faster launches, classroom-proof) instead of chasing LAN DNS.

## Seed (observed)

- Enumerated Fuel URIs across all turtlebot3_gazebo worlds: exactly
  2 (Ground Plane v5, Sun v3).
- `gz fuel download -u <url>` in a base-image container → 68K cache.
- Copied to `images/workspace/home-seed/.gz/fuel/...` (64K):
  `init-student-home`'s existing `cp -rn` seeds it into
  `$HOME/.gz` on first boot (image and pod paths — no script
  changes; the Dockerfile `chown` already covers the dir).
- Rebuilt workspace green (cache-hot, seconds).

## Offline proof (observed, `--network none` container)

- `turtlebot3-sim empty`: gz sim alive, all bridges created
  (clock, joint_states, odom, tf, cmd_vel, imu, scan).
- `gz topic -l`: full bus (`/clock`, `/stats`, world/model topics).
- ROS side: `/clock` **158.882 → 159.930** across consecutive
  echoes ≈ real-time for the empty world on llvmpipe; gz sim ~90%
  of one core.
- Test container removed; live session stopped (`stopped`).

## Dirty-tree discipline

New: `images/workspace/home-seed/.gz/` (seed data) + this report
(staged for commit). The 8 join/nodes files + `join-cluster.sh`
untouched/unstaged; untracked junk left alone.

## Next

New immutable tag + `docker save` → operator `sudo k3s ctr images
import` → live re-run (`empty` world in-pod, same proofs) →
checkpoint. The seed set covers `world`/`house` too (same 2 URIs).
Still queued: sim viewer UI (SPA, other branch), merge, R2-final,
pre-warm/registry, GPU follow-up.
