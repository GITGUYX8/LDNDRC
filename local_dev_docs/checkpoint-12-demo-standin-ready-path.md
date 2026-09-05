# Checkpoint 12 — Stand-In Image Proves the Ready Path

## The big picture first

Every gateway test so far ended at 503: no session ever became `ready`
because the workspace image doesn't exist. That left the most important path
— a logged-in user actually reaching their workspace through the browser
hosts — completely unverified. This checkpoint closed that gap with a tiny
stand-in image (82MB): a Python HTTP server that answers the readiness probe
and identifies itself on every response.

Think of it as a cardboard cutout in the shop window: it proves the window,
the lighting, and the foot traffic work. Nobody mistakes it for merchandise.

## What got built

| File | Friendly name | What it does |
|---|---|---|
| `images/demo-standin/Dockerfile` | The Cardboard Cutout | Builds the 82MB stand-in image on `python:3.14-alpine`. |
| `images/demo-standin/server.py` | The Greeter | Answers 200 on 7682 with a "plumbing only" message; logs silenced. |
| `manifests/control-panel-deployment.yaml` | The Demo Switch | `SESSION_IMAGE` points at the stand-in, commented as demo-only. |
| `local_dev_docs/checkpoint-12-*.md` | This Story | Records the first end-to-end 200s, scoped honestly. |

No Go code changed. The provisioner already took the image from env.

## How each piece works

### The Greeter — `server.py`

A stdlib `HTTPServer` on port 7682 (the session editor port the readiness
probe hits). Every response says `ldndrc-demo-standin: plumbing only`, so a
green dashboard can never be misread as a working ROS workspace. Ports
8080/9002 intentionally serve nothing — desktop/gazebo traffic should fail
with a clean 502, which is itself a verified behavior.

### The Demo Switch — deployment manifest

`SESSION_IMAGE: ldndrc/demo-standin:dev` with a comment naming the
production image. The code default stays
`ldndrc/ros2-gz:jazzy-harmonic-workspace`; only this demo manifest
overrides it. Reverting is one line.

## How it all connects

1. Stand-in built (82MB), imported into k3d, deployment applied + restarted.
2. Stale `ImagePullBackOff` session deleted; fresh session created via API.
3. Scheduler places the pod; local-path binds the PVC; image pulls from the
   node (already imported, no registry needed).
4. Readiness probe passes → pod 1/1 Running → store promotes session to
   `ready` on first gateway lookup.
5. Gateway proxies the editor host to the session Service → 200 with the
   stand-in body.

## Proof it works

Observed live:

- Pod `ros2-session-5ed57ac3be64-...` 1/1 Running.
- Session status `provisioning` → `ready` (promotion timestamp recorded).
- Gateway editor host + cookie → **200** with the stand-in body, via both
  port-forward (`:8082`) and Traefik (`:80`).
- Gateway desktop/gazebo hosts + cookie → **502** (nothing listening —
  correct stand-in behavior, and proves the error path returns cleanly).
- Gateway editor host without cookie → **401** (auth still enforced).
- Session list still scoped to the owning user.

## The one-liner version

| Piece | One-liner |
|---|---|
| Stand-in image | A cardboard cutout that lets the ready path go green. |
| Gateway 200s | Traefik → cookie → session Service routing works end to end. |
| 502s and 401 | The failure paths behave exactly as designed, too. |

## What's next

- [ ] Replace the stand-in with the real Jazzy/Harmonic workspace image on
      GPU hardware (the true end state).
- [ ] Remove legacy `ros2-ingress.yaml` path routes — now unblocked, since
      the host-based ready path is proven.
- [ ] Session store persistence (restarts still orphan K3s resources).
- [ ] CSRF protection on state-changing browser requests.
- [ ] Student frontend UI against the verified API.
- [ ] Commit this checkpoint's files when ready.
