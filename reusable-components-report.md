# Reusable Components Report

**Purpose:** Inventory of production-tested components available in the local reference implementation (`../A-Scalable-Cloud-Native-Platform-for-ROS-Education-mvp1-selkies/`) that LDNDRC can adopt for Phase 3 (ROS2, Gazebo & Web Integration), instead of building everything from scratch.

**Scope:** Technology and configuration only. The reference implementation targets the same stack as LDNDRC — ROS2 Jazzy, Gazebo Harmonic, k3s, Selkies, browser-based access — and its components have been run in a production-shaped deployment.

---

## 1. Component Inventory

| Component | Location | What it provides | Reuse level |
|---|---|---|---|
| ROS2 + Gazebo base image | `apps/student-workspace/base/` | `Dockerfile` on `osrf/ros:jazzy-desktop-full-noble` + `installs.sh` with a complete, tested apt package list (`ros-jazzy-ros-gz`, `ros-jazzy-ros-gz-bridge`, `ros-jazzy-turtlebot3-gazebo`, Nav2, colcon, dev tools). Pre-sets `ROS_AUTOMATIC_DISCOVERY_RANGE=LOCALHOST` — the same localhost-only DDS isolation LDNDRC designed. | Near-direct copy |
| Browser workspace image | `apps/student-workspace/minimal/` | `Dockerfile` layering on the base: Selkies-GStreamer 1.6.2 (WebRTC desktop streaming), code-server (browser IDE), supervisord (process supervision), VirtualGL (GPU-accelerated GL), IceWM desktop. Exposes 7682 (editor), 8080 (desktop), 9002 (Gazebo websocket). | Near-direct copy |
| Selkies installer | `apps/student-workspace/minimal/install-selkies.sh` | 200-line tested install script pulling the upstream Selkies release artifacts with the correct GStreamer/codec combination. Avoids hand-assembling a fragile codec stack. | Direct copy |
| Gazebo websocket bridge config | `apps/student-workspace/minimal/websocket.gzlaunch` | The `gz-launch-websocket-server` plugin config: port 9002, `publication_hz=30`, `max_connections=-1`. More complete than plan.md's current draft (which has `max_connections=1` and no publication rate). | Direct copy |
| Process supervision config | `apps/student-workspace/minimal/supervisord.conf` | Runs three supervised programs — code-server, Selkies, gazebo-web — each with `autorestart=true`. Handles startup ordering and crash recovery inside the container. | Direct copy |
| Start scripts | `apps/student-workspace/minimal/bin/` | `start-selkies`, `start-code-server`, `start-gazebo-web`, `turtlebot3-sim`, `run-with-virtualgl`, `init-student-home`. Encapsulate the environment-variable wiring. | Direct copy |
| gzweb browser viewer | `tests/gazebo-websocket-viewer/` | Working Vite app using the `gzweb` npm package: creates a `SceneManager`, connects it to the websocket URL, renders the scene in WebGL. Includes a debug panel and auto-reconnect. | Direct copy (becomes sim-web-app's UI) |
| Session manifest template | `scripts/k8s/k3s/base/session-template.yaml` | Deployment + Service + home PVC per session, with readiness probes, `runAsUser: 1000`, `allowPrivilegeEscalation: false`, resource requests/limits, and named container ports. | Adapt patterns (probes, security, ports) into our StatefulSet |
| k3s operator scripts | `scripts/k8s/k3s/` | `deploy.sh`, `cluster-setup.sh`, `load-images.sh`, runbooks for server setup, worker nodes, NFS storage. | Reference for Phase 2+ operations |
| TURN relay (coturn) | `apps/turn-server/`, `scripts/k8s/k3s/base/coturn-template.yaml` | WebRTC media relay for when direct browser↔pod media paths fail (restrictive NAT/firewall). | Optional — defer |
| Control panel (auth + session API) | `backend/`, `frontend/` | NestJS + React + Postgres stack for user login and on-demand session creation. **Behavior reused, language replaced: Go backend (client-go, JWT, ReverseProxy).** | Not adopted as-is — reimplemented in Go (see §5) |

---

## 2. Decision Point: Pod Layout — 1 Container vs 4 Containers

The reference implementation runs **one container per session**, with supervisord managing the editor, desktop stream, and Gazebo bridge inside it. LDNDRC's plan.md currently specifies **four separate containers** in one pod (ros2-gazebo, sim-web-app, selkies, monaco) sharing the pod network namespace.

| Aspect | 1 container + supervisord (reference) | 4 containers per pod (current plan) |
|---|---|---|
| Production proof | Yes — this exact layout has run in a real k3s deployment | Not yet proven in this stack |
| Images to build/maintain | 1 (base) + 1 (workspace layer) | 3-4 separate images |
| Startup ordering | Handled by supervisord priorities | Kubernetes gives no in-pod ordering guarantees |
| Crash recovery | supervisord `autorestart` | kubelet restarts the individual container |
| Failure isolation | One process crash doesn't kill the others (supervisord restarts it), but a container-level failure kills everything at once | A crashed container restarts alone; others keep running |
| Resource accounting | One budget for the whole workspace | Per-service requests/limits (finer-grained) |
| Kubernetes idiomaticity | Less (supervisord duplicates kubelet's job) | More (one concern per container) |
| DDS isolation | All processes share the container's localhost — trivially satisfied | Satisfied via shared pod network namespace |
| Image size concerns | Single large image pulled once | Smaller individual images, but 4 pulls per pod |

**Summary:** The single-container layout is the lower-risk path — it is already proven with this exact software combination, needs fewer images, and supervisord solves the startup-order problem that raw Kubernetes does not address inside a pod. The 4-container layout is more idiomatic and gives finer failure isolation and resource accounting, at the cost of being unproven here and requiring more build plumbing.

**This report presents both; the choice is left open.** Either way, every component in §1 (base image, Selkies installer, websocket config, gzweb viewer, manifest patterns) remains reusable.

---

## 3. Decision Point: Browser Editor — code-server

**Recommendation: adopt code-server; drop Monaco from the plan.**

| Aspect | code-server (reference) | Monaco standalone (current plan) |
|---|---|---|
| What it is | Full VS Code running in the browser | Just the editor widget (VS Code's text-editing core) |
| Integrated terminal | Yes — students run `ros2 launch`, `gz sim` directly in the browser | No — terminal would need a separate component |
| Extensions | VS Code extensions supported | None |
| Integration effort | Already wired into the reference image, supervised, port assigned | We would have to build serving + file wiring ourselves |
| Image weight | Heavier (~100MB) | Lighter |

The integrated terminal is the deciding factor: LDNDRC's user flow requires students to launch simulations and run ROS2 commands. Monaco alone cannot do that; code-server does it out of the box. plan.md's Monaco container becomes unnecessary.

---

## 4. Decision Point: ROS_DOMAIN_ID Strategy

| Approach | How it works | Assessment |
|---|---|---|
| Static domain everywhere (reference) | Every workspace uses `ROS_DOMAIN_ID=30`; isolation comes purely from `ROS_AUTOMATIC_DISCOVERY_RANGE=LOCALHOST` + per-pod network namespaces | Works — DDS physically cannot leave localhost, so identical domains cannot cross-discover |
| Per-pod domain (current plan) | StatefulSet ordinal → `ROS_DOMAIN_ID = 100 + ordinal` | Same physical isolation, plus defense-in-depth: even if a packet somehow escaped localhost, domains wouldn't match |

**Recommendation: keep LDNDRC's per-ordinal approach.** It costs almost nothing (one line in the entrypoint reading the pod name) and buys a second isolation layer. The reference's localhost discovery-range setting (`ROS_AUTOMATIC_DISCOVERY_RANGE=LOCALHOST`) should *also* be adopted — the two compose, they don't compete.

---

## 5. Deliberately Not Picked

| Component | Why not |
|---|---|
| Control panel **as NestJS + React + Postgres + Prisma** | The reference's session-management stack is Node-based and heavy for laptop hardware. **Decision: build the control-plane backend in Go instead** (client-go, JWT, net/http reverse proxy, single static binary ~20MB). The frontend stays browser-native JS (code-server, gzweb viewer, Selkies) — Go cannot replace WebGL/WebRTC components. A minimal Go-served control page (embed.FS or htmx) covers the landing/launch UI. See plan.md Phase 5. |
| TURN server (coturn) | Only needed when WebRTC direct media paths fail (restrictive NAT). On a LAN — LDNDRC's stated target — direct paths work. Defer until a real deployment shows media failures. |
| Backend gateway / WebSocket proxy in Node | The reference used http-proxy in NestJS. A Go equivalent uses `net/http/httputil.ReverseProxy` — same capability, static binary. |
| Minikube/EKS scripts | Historical experiments, not the production path. |

---

## 6. Pick List for Phase 3

Ordered, concrete, ready to execute:

- [ ] **Base image:** copy `apps/student-workspace/base/Dockerfile` + `installs.sh` → `LDNDRC/images/base/`. Build as `ldndrc/ros2-gz:jazzy-harmonic-base`.
- [ ] **Workspace layer:** copy `apps/student-workspace/minimal/` (Dockerfile, `install-selkies.sh`, `supervisord.conf`, `bin/`, launch files) → `LDNDRC/images/workspace/`. Adapt the base-image reference to point at our base tag.
- [ ] **Websocket config:** copy `websocket.gzlaunch` verbatim (port 9002, 30 Hz, unlimited connections). Replaces plan.md's draft.
- [ ] **Entrypoint:** add per-pod `ROS_DOMAIN_ID` derivation (pod name ordinal + 100) and confirm `ROS_AUTOMATIC_DISCOVERY_RANGE=LOCALHOST` is set — both isolation layers.
- [ ] **gzweb viewer:** copy `tests/gazebo-websocket-viewer/` → `LDNDRC/sim-web-app/`. Wire its websocket URL to the pod's port 9002. Note its documented requirements: Node 24+, `@babel/runtime` as a direct dependency, and `SceneBroadcaster` in any SDF world that should stream scene data.
- [ ] **Manifest:** write `ros2-platform.yaml` StatefulSet using the reference session-template's patterns — readiness probes, `runAsUser: 1000`, `allowPrivilegeEscalation: false`, named ports (editor 7682, desktop 8080, gzweb 9002) — plus LDNDRC's own `nodeSelector: node-role.kubernetes.io/role: host`.
- [ ] **Service + Ingress:** extend the proven test-svc/test-ingress pattern to the three named ports and paths (`/code` → 7682, `/stream` → 8080, `/sim` → 9002).
- [ ] **Cluster rehearsal:** deploy on the k3d test cluster, verify pods land on the Host-labelled node, verify `/code`, `/stream`, `/sim` routes end-to-end, then write checkpoint-02.

---

## 7. Impact on plan.md

Once the pod-layout decision (§2) is made, plan.md needs these updates:

1. **Tech stack table:** Monaco Editor → code-server.
2. **Architecture diagram + Phase 3 manifest:** reflect chosen pod layout (1 container + supervisord, or 4 containers).
3. **`websocket.gzlaunch` snippet:** update to port 9002, `publication_hz=30`, `max_connections=-1`.
4. **Add gzweb viewer as a named component** (the Vite app), replacing the vague "nginx:alpine serves gzweb npm" placeholder.
5. **Add `ROS_AUTOMATIC_DISCOVERY_RANGE=LOCALHOST`** to the isolation model section as an explicit second layer alongside per-ordinal domains.
