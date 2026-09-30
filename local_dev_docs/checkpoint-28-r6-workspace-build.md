# Checkpoint 28 — R6 Workspace Build (Selkies deferred) (detailed)

## Context

Second R6 chunk on `feat/r6-workspace-image`: build `images/workspace`
on top of the checkpoint-27 base. Hit a real upstream breakage
mid-build, resolved per operator decision. No panel/backend changes.

## Upstream breakage: Selkies 1.6.2 assets gone

- Build failed in `install-selkies.sh` step 1: the GStreamer tarball
  `gstreamer-selkies_gpl_v1.6.2_ubuntu24.04_amd64.tar.gz` returns 404
  (VirtualGL 3.1.4 just above it downloaded fine).
- Investigation: `v1.6.2` tag itself 404s; the releases API lists only
  `2.0.0` (+ 2 rcs, Sep 2026) with a restructured asset layout (per-OS
  `.deb`s, `selkies-2.0.0-py3-none-any.whl` — no GStreamer tarball, no
  `selkies_gstreamer` wheel, no js-interposer deb). PyPI has `selkies`
  2.0.0 but no `selkies-gstreamer` package. The 1.x line is
  unrecoverable from upstream.
- Operator decision: **defer Selkies**, ship editor + sim first.

## Deferral implementation (reversible, in-tree)

- `images/workspace/Dockerfile`: new `ARG WITH_SELKIES=0`; the
  installer RUN is conditional (skips with a log line at 0, installs
  at 1). `install-selkies.sh`, `start-selkies.sh`, and the
  `SELKIES_*` env stay in-tree for the 2.0.0 migration chunk.
- `images/workspace/supervisord.conf`: `[program:selkies]` section
  removed with a comment (an installed-but-absent program would
  restart-loop under `autorestart`). Migration restores it.
- Runtime effect: nothing listens on 8080 → gateway `desktop` stays
  **502-by-design**, exactly like the demo stand-in. No provisioner
  change needed (readiness only probes 7682).

## Build (observed)

Rebuilt green after the deferral: code-server install, Node 24,
`npm install gzweb` + audit notes, seed/home setup, export ~30s.

Result: `ldndrc/ros2-gz:jazzy-harmonic-workspace` — **10.2GB**
(index `cc54d71348fb`, config `eaaf8088…`) on the 8.25GB base.
Matches the roadmap's ~10GB+ expectation.

## Dirty-tree discipline

Only `images/workspace/Dockerfile` + `supervisord.conf` modified
(staged for this chunk's commit). The 8 join/nodes files +
`join-cluster.sh` untouched/unstaged; untracked junk left alone.

## Next chunk

Local contract verify (`docker run`: 7682/8080/9002 answers, probe
path ≤399, uid 1000, supervisord keeps code-server + gazebo-web up)
→ checkpoint-29. Then immutable tag + import → `SESSION_IMAGE`
flip → live matrix. Queued separately: Selkies 2.0.0 migration
(`ubuntu24.04` deb layout, start-script + silent-audio-patch
adaptation, conf restore, `WITH_SELKIES=1`).
