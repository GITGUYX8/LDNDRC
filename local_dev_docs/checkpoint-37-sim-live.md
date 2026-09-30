# Checkpoint 37 — Sim Smoke Live Re-run (detailed)

## Context

R6 sim finale on `feat/r6-workspace-image`: seeded image
(`10f1f10c`) imported live, panel flipped, full in-pod proof.
Follows checkpoint-36 (seed + offline proof).

## Import + flip (observed)

- Operator `sudo k3s ctr images import
  ~/ldndrc-workspace-jazzy-harmonic-10f1f10c.tar`: index
  `10f1f10c…`, 28s (layer sharing with the previous tag).
- Live `SESSION_IMAGE` patched to the seeded tag via `kubectl set
  env`; committed manifest + README bumped to match.
- Delete-old-first panel restart → new pod Running (in-memory
  sessions wiped, known R2 debt).

## Live proof (session `6d80a7ef40bf`, seeded image)

- Pod runs `ldndrc/ros2-gz:jazzy-harmonic-10f1f10c`, `1/1 Running`.
- Seed present in pod HOME (`ground plane`, `sun` listed).
- `turtlebot3-sim empty`: gz sim alive, **zero** "Unable to find
  uri" errors, all bridges created (clock, joint_states, odom, tf,
  cmd_vel, imu, scan).
- ROS bus full: `/clock /cmd_vel /imu /joint_states /odom /scan
  /tf(/_static) /robot_description`.
- `/clock` **347.948 → 349.013** across consecutive echoes ≈
  real-time for the empty world on CPU-only; gz sim ~93% of one
  core. (One thin `ros2 topic list` mid-run was CLI discovery lag,
  not missing topics — retry showed the full bus.)
- Session stopped (`stopped`) afterwards.

## Dirty-tree discipline

Only manifest tag bump + README + this report (staged for commit).
The 8 join/nodes files + `join-cluster.sh` untouched/unstaged;
untracked junk left alone.

## What remains

Sim track done (CP-36…CP-37): sim runs offline-safe at ≈RTF for
the empty world. Still queued: sim viewer UI (SPA, other branch),
merge with `feat/r3-csrf`, R2-final, join-time image pre-warm /
registry, GPU follow-up, CSRF login-lockout fix (planned,
`feat/r3-csrf`).
