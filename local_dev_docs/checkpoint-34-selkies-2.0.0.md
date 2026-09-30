# Checkpoint 34 — Selkies 2.0.0 Migration (detailed)

## Context

R6 follow-up on `feat/r6-workspace-image`: migrate the deferred
Selkies install to upstream 2.0.0 (the 1.6.2 assets were deleted).
No panel/backend changes.

## What 2.0.0 changed (found by inspecting the .deb)

- Pure-Python WebRTC/WebSockets stack (`selkies.webrtc`, aiortc-style):
  **no GStreamer at all** — no gst tarball to download, no gst-env to
  source, no NVENC plugin build.
- Self-contained `/opt/selkies` venv (python3.12) + `/usr/bin/selkies`
  entry; HTML5 client ships inside (`selkies/selkies_web/`).
- Settings resolve CLI flag > `SELKIES_<NAME>` env > default; dash
  form (`--audio-enabled`, `--enable-basic-auth`).
- Audio off is a **native flag** (`audio_enabled=false`) — the 1.x
  pulsesrc→audiotestsrc source patch has no target and is dropped.
- Encoder names are codecs (`h264enc` default, software without GPU),
  not GStreamer elements — the `nvh264enc`/`x264enc` mapping is gone.

## Changes (4 files)

- `images/workspace/install-selkies.sh`: rewritten — X stack +
  deb Depends + VirtualGL (kept, no-op without GPU) via apt, then
  `selkies-2.0.0-ubuntu24.04-amd64.deb` (59MB) via apt install,
  `selkies --version` assertion. Dropped: GStreamer tarball, wheel +
  venv build, web tarball, js-interposer, audio patch.
- `images/workspace/bin/start-selkies.sh`: Xvfb/IceWM setup kept
  (still valid); launch rewritten for 2.0.0 flags — auth/TLS off
  (gateway owns them), audio off, clipboard/resize/bitrate/framerate
  mapped, TURN/STUN in dash form, `auto` encoder omits the flag,
  `SELKIES_MODE` override passthrough. Dead GPU-detection and stale
  `encoder`/`WEB_ROOT` references removed.
- `images/workspace/Dockerfile`: `SELKIES_VERSION=2.0.0`,
  `WITH_SELKIES=1` default; removed the 1.x-only
  `WEB_ROOT`/`VENV`/audio-bitrate envs.
- `images/workspace/supervisord.conf`: `[program:selkies]` restored.

## Proofs (local `docker run`, no cluster)

- `selkies --version` → `2.0.0`; image 10.2 → **10.8GB**.
- All three programs RUNNING past startsecs.
- `:8080/` → **200 Selkies HTML client** (was refused).
- Startup summary: `websockets transport, X11 (:50) capture,
  encoder h264enc, audio off, access open` — every flag parsed.
- `:7682/` still 302 → 200 IDE (unchanged).

## Dirty-tree discipline

Only the 4 files above + this report (staged for commit). The 8
join/nodes files + `join-cluster.sh` untouched/unstaged; untracked
junk left alone. Test container removed.

## Next

New immutable tag + `docker save` → operator `sudo k3s ctr images
import` → live session matrix with desktop 200 (no longer 502) →
checkpoint. Then: merge with `feat/r3-csrf`, R2-final, join-time
image pre-warm.
