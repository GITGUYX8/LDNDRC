# Checkpoint 02 — The Workspace Containers & Per-Pod Isolation

## The big picture first

The farm has proven its ground rules — only strong laptops join, workloads land
only on Host machines, and a browser can reach them. But so far we rehearsed
with toy passengers (a one-line web page). This checkpoint built the real
vehicle: the actual container image that will carry the robot workspace —
Gazebo simulation, browser editor, desktop streaming — plus the little switch
that gives every student their own private radio channel so nobody's robots
accidentally talk to someone else's.

## What got built

| File | Friendly name | What it does |
|---|---|---|
| `images/base/Dockerfile` + `installs.sh` | The Foundation | A ready-made ROS2 + Gazebo workshop, pre-loaded with TurtleBot3 and tools |
| `images/workspace/` (82 files) | The Workspace Kit | Everything a browser workspace needs: editor, desktop streaming, simulation bridge |
| `images/workspace/bin/entrypoint.sh` | The Channel Assigner | Gives each workspace its own private radio channel and locks traffic to its own room |
| `images/workspace/Dockerfile` | The Assembly Line | Puts the Workspace Kit together on top of The Foundation |

## How each piece works

### The Foundation — `images/base/`

The base container is the "factory default" laptop for the farm. We reused a
tried-and-tested recipe: the standard ROS2 Jazzy full desktop image, plus a big
shopping list of extras — Gazebo simulation, the bridge that lets ROS2 talk to
Gazebo, TurtleBot3's robot packages, and coding tools like colcon and clang.
It also switches on a safety setting (DDS only talks on localhost) so no robot
data can wander outside its own container.

### The Workspace Kit — `images/workspace/`

On top of the foundation, this kit adds the three things a student sees in a
browser:

- **code-server** — a full editor (VS Code) with a built-in terminal, so
  students can type `ros2 launch` right in the browser.
- **Selkies** — the live desktop streaming, so the whole Linux desktop shows up
  in a browser tab.
- **gazebo-web** — the bridge that pipes the 3D simulation view out through a
  WebSocket to the browser.

A tiny boss process (supervisord) watches all three and restarts any that crash.
Each piece ships with its own startup script, all port-tested: editor on 7682,
desktop on 8080, simulation view on 9002.

### The Channel Assigner — `images/workspace/bin/entrypoint.sh`

This is the new brain we added. Every workspace pod is named after a number —
`ros2-platform-0`, `ros2-platform-1`, and so on. At startup, the Channel
Assigner reads that number and sets the workspace's radio channel to `100 + the
number`. So pod 0 gets channel 100, pod 1 gets 101, pod 7 gets 107. Because
every workspace is on a different channel, a robot in one pod can never hear a
robot in another — even on the same laptop. It also re-locks the localhost-only
setting, so discovery traffic stays inside the pod's own room. This is the
defense-in-depth that makes "fully isolated per student" a guarantee, not a hope.

### The Assembly Line — `images/workspace/Dockerfile`

This is the instruction manual that builds the final image: take The Foundation,
install the Workspace Kit, plug in the Channel Assigner, and tell the container
to start everything through it.

## How it all connects

```
The Foundation (ROS2 + Gazebo + TurtleBot3)
        │
        ▼
The Assembly Line layers on the Workspace Kit
(code-server editor + Selkies desktop + gazebo-web bridge, watched by supervisord)
        │
        ▼
Container starts → Channel Assigner sets the private radio channel
        │
        ▼
Supervisord keeps all three services alive:
  editor on 7682 · desktop on 8080 · simulation view on 9002
```

## Proof it works

- **Syntax checks:** all copied shell scripts (`install-selkies.sh`,
  `installs.sh`, and the 10 `bin/` scripts) pass clean `bash -n` checks — no
  typos that would stop them running.
- **Launch file:** the TurtleBot3 web launch file parses as valid Python.
- **Channel Assigner, 4/4 scenarios:**
  - pod `ros2-platform-0` → channel 100
  - pod `ros2-platform-7` → channel 107
  - no pod name (local testing) → falls back to channel 30
  - pod name with no number → falls back to channel 30
- The `FROM` reference in the workspace image was correctly repointed at our own
  base image, so the build is fully self-contained.
- Not yet verified: the images have not been built or run on a cluster — that's
  the very next step.

## The one-liner version

| Piece | One-liner |
|---|---|
| `images/base/` | The pre-loaded ROS2 + Gazebo + TurtleBot3 workshop |
| `images/workspace/` | The browser workspace: editor + desktop stream + simulation bridge |
| `bin/entrypoint.sh` | Gives each workspace a private channel + locks it to its own room |
| `images/workspace/Dockerfile` | Assembles everything into one container |

## What's next

- [ ] Copy the gzweb browser viewer (the 3D scene renderer) into `sim-web-app/`
- [ ] Write the real farm manifest (StatefulSet + Service + Ingress) with the
      proven Host-node seating rule
- [ ] Build the images (check disk space first) and deploy them on the test
      cluster
- [ ] Rehearse the browser paths (`/code`, `/stream`, `/sim`) end to end
