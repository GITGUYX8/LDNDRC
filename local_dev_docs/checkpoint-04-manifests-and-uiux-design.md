# Checkpoint 4 — The Private Pod, The Sticky Door, The Front Door Sign, and the Design Sketch

## The big picture first

The platform's heart — the workspace image — was already built and proven in
previous checkpoints. What was missing was the *wiring*: a way for the cluster
to run one private workspace per student, keep each student stuck to their own
workspace, and let a browser reach it. This checkpoint wrote all three pieces of
that wiring and — separately — drew the first real design sketch for what
students will actually *see* when they open their browser. Nothing was deployed
yet; the wiring is written and checked, ready to be tested for real once the
workspace image is built.

## What got built

| File | Friendly name | What it does |
|---|---|---|
| `manifests/ros2-platform.yaml` | The Private Pod | Tells the cluster to run 3 private workspaces, one per pod, each on its own private radio channel |
| `manifests/ros2-service.yaml` | The Sticky Door | Gives the pods one stable address that remembers which browser belongs to which pod |
| `manifests/ros2-ingress.yaml` | The Front Door Sign | The address book that points browser paths (`/code`, `/stream`, `/sim`) at the right door |
| `uiux-design.md` | The Design Sketch | The first plan for what the student-facing screens look and feel like |

## How each piece works

### The Private Pod — `manifests/ros2-platform.yaml`

Think of it as an apartment block with one tenant per unit. The block manager
(the StatefulSet) names each unit `ros2-platform-0`, `ros2-platform-1`,
`ros2-platform-2` — and that number is the whole trick. The workspace image's
entrypoint reads that number and sets its radio channel to `100 + number`, so
pod 0 hears only channel 100, pod 1 only channel 101, and so on. Two robots in
different pods can never accidentally talk to each other, because they're
literally on different channels and DDS traffic is pinned to the local
machine only.

The pod also wears a name badge that only Host laptops recognize (a
`nodeSelector` — a rule that says "only put me on a laptop wearing the Host
badge"), so these heavy workspaces always land on the GPU/performance machine.
Each pod watches itself with a readiness check: it pokes its own editor port
every 10 seconds, and only claims "I'm ready" once the editor answers.

### The Sticky Door — `manifests/ros2-service.yaml`

This is the "mailbox" that gives all three pods a single stable address. The
clever bit is the sticky part: once a browser first knocks, the door remembers
which browser came in (by remembering their IP address for an hour) and keeps
sending that browser's traffic to the same pod every time. That matters because
a student's editor, simulation, and desktop are all running on *their own* pod —
if the door randomly shuffled traffic between pods, you'd lose your work mid-task.

### The Front Door Sign — `manifests/ros2-ingress.yaml`

The address book on the building's front entrance. It reads the URL a guest
types in (`ros-platform.local/code`) and sends it to the right door: `/code`
goes to the editor, `/stream` to the desktop stream, `/sim` to the simulation
websocket. The sign itself is a copy of the smaller test sign we already proved
works in Checkpoint 1 — same style, same host name, just pointing at the real
apartments instead of the test room.

### The Design Sketch — `uiux-design.md`

Before this checkpoint, students would have faced three unrelated websites with
no obvious connection. This document plans the fix: one branded, dark-themed
"mission control" look, a login page, a launch page that shows live progress as
the workspace boots, and a workspace screen where Code and Sim sit side by side.
It deliberately drops the reference project's floating-window gimmick in favor
of one focused stage you can switch between — because students should be
learning ROS, not arranging windows. This is a *plan*, not code yet; the
implementation comes in a later phase.

## How it all connects

1. A student opens the browser → hits the Front Door Sign (`ros2-ingress.yaml`).
2. The sign reads the path (`/code`, `/stream`, or `/sim`).
3. The Sticky Door (`ros2-service.yaml`) remembers which pod that browser belongs to.
4. Traffic lands in the student's Private Pod (`ros2-platform.yaml`).
5. Inside the pod, the entrypoint has already assigned that pod its own private
   channel (ROS_DOMAIN_ID = 100 + pod number), so their ROS nodes are isolated
   from every other student.
6. The Design Sketch shows how all of this is presented to the student as one
   clean, friendly product instead of three raw tools.

## Proof it works

The three wiring files were checked twice against the live cluster — once
locally and once against the real Kubernetes API — and both times the cluster
accepted them without a single complaint:

```
statefulset.apps/ros2-platform created (dry run)      # client-side check
service/ros2-platform-svc created (dry run)
ingress.networking.k8s.io/ros2-platform-ingress created (dry run)
# ...and the server-side re-check passed with the same three "created (dry run)" lines
```

The actual deployment has **not** happened yet — that needs the workspace image
to be built first (next task). One thing to watch for when we do deploy: the
small test sign from Checkpoint 1 already claims the `/sim` path, so it will
need to be removed first to make room for the real one.

## The one-liner version

| Piece | One-liner |
|---|---|
| `manifests/ros2-platform.yaml` | The apartment block that gives each student their own isolated, numbered workspace pod |
| `manifests/ros2-service.yaml` | The sticky door that keeps each browser glued to its own pod |
| `manifests/ros2-ingress.yaml` | The address book that routes `/code`, `/stream`, and `/sim` to the right place |
| `uiux-design.md` | The first plan for turning three raw tools into one friendly, dark-themed product |

## What's next

- [ ] Task 6: update `tech-enhancement-report.md` so it matches the decisions already made in `plan.md` (code-server, Go control panel, corrected websocket config)
- [ ] Task 7: build the workspace image — check disk space first, then build base → workspace
- [ ] Task 8: deploy on k3d, remove the old test sign, rehearse all three routes, then write the next checkpoint