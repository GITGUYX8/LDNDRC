# Tech Enhancement Report — Distributed ROS2 Robotics Platform on Laptop Cluster

**Date:** July 13, 2026 (revised)
**Evaluation Lens:** This project is a greenfield local-first, browser-accessible robotics simulation platform built on a dynamic Kubernetes (K3s) cluster running across consumer laptops. It targets educational/institutional settings with no dedicated server hardware, aiming for <50 concurrent users running ROS2/Gazebo simulations. The platform must be self-hosted on LAN (Wi-Fi/Ethernet), zero cloud dependency, deployable by non-experts, and tolerant of laptops joining/leaving the cluster dynamically. Reliability, simplicity of setup, and tolerance for consumer-grade hardware are more important than raw throughput or feature breadth.

> **Revision notes:** 
> 1. **Flannel / DDS multicast** — An earlier draft incorrectly claimed Calico's VXLAN mode would improve DDS multicast performance. Verified against Calico's overlay docs — Calico VXLAN is unicast-only like Flannel. **However**, the architecture has since shifted to isolation-first: each pod is a self-contained ROS2 island with all containers sharing localhost. DDS traffic **never leaves the pod**, making the Flannel multicast issue completely irrelevant. No `hostNetwork`, no Fast DDS TCP transport, no CNI swap needed.
> 2. **User isolation** — Added `ROS_DOMAIN_ID` per pod (derived from StatefulSet ordinal) for defense-in-depth. Flannel carries only HTTP, WebSocket, and WebRTC traffic.
> 3. **Silkies → Selkies** — Correct project name is **Selkies** (selkies-project/selkies, 1.9k stars). Recommendation flipped from "replace with noVNC" to "keep it, fix spelling, add GPU config."
> 4. **Gzweb → Gazebo Sim `websocket_server` plugin** — Replaced legacy gzweb container with modern `gz-sim` websocket plugin + lightweight gzweb npm web app.
> 5. **Gazebo version matrix** — Added: Harmonic LTS (EOL 2029) pairs with both ROS2 Humble and Jazzy. Avoid Jetty.
> 6. **k0s removed** — Evaluated but offers no benefit for this use case. K3s stays.

---

## Executive Summary

- **Selkies is a great fit — no replacement needed** — The plan misspelled the name as "Silkies"; the real project is **Selkies** (selkies-project/selkies, 1.9k stars, updated July 2026). It's an actively maintained low-latency WebRTC HTML5 streaming platform with native NVIDIA GPU acceleration and first-class Kubernetes support. Keep it, just fix the spelling.
- **HIGH: ROS2 Humble is an aging LTS** — Jazzy (May 2024) is now the current LTS, with support through 2029. Starting with Jazzy buys 2+ more years of support and newer DDS features. Migration cost is low at design stage.
- **RESOLVED: Flannel does not support DDS multicast, but it doesn't matter** — each pod is a fully isolated ROS2 island. The workspace runs as a **single container** (code-server, Selkies, gazebo-web under supervisord) sharing the pod network namespace. DDS discovery and topic traffic **stay on localhost** and never traverse the Flannel overlay. Flannel only carries HTTP, WebSocket, and WebRTC traffic — all unicast, all fine.
- **MEDIUM: No observability, auth, or monitoring planned** — the platform lacks health checks, logging aggregation, rate limiting, and any authentication. For a platform running arbitrary user code (ROS2 nodes), this is a security concern even in a LAN setting.
- **MEDIUM: No CI/CD, testing, or container build pipeline** — the plan has no Dockerfiles, no CI, no testing strategy. These should be defined before implementation.
- **MEDIUM: Gazebo version ambiguity** — the plan references `gzweb` which is part of the legacy Gazebo Classic (Ignition) ecosystem. The current Gazebo Sim (`gz-sim`) ships a `websocket_server` system plugin. Target **Gazebo Harmonic** (LTS through 2029) — it pairs with both ROS 2 Humble and Jazzy. Avoid Jetty (newest, but it's an interim release without the ROS2 LTS pairing).
- **REMOVED: k0s evaluation** — Previously suggested as a lighter K3s alternative, but k0s adds setup overhead (manual CNI, etcd, no bundled ingress) without solving any project bottleneck. K3s is the correct choice.

---

## Current Stack Inventory

*Note: This project has zero code written. The following is an assessment of the **planned** tech stack from `plan.md`.*

### Core Framework (Orchestration)
- **Current:** K3s (lightweight Kubernetes)
- **Usage:** Central orchestrator — cluster management, pod scheduling, node lifecycle
- **Assessment:** Excellent fit. K3s is the gold standard for lightweight K8s on edge/IoT hardware. 33.5k stars, CNCF sandbox, latest v1.36.2+k3s1 (June 2026). The bundled components (Traefik, Flannel, CoreDNS, local-path-provisioner) align well with the use case.

### Container Runtime
- **Current:** Containerd (bundled with K3s)
- **Assessment:** Correct choice. Lightweight, Kubernetes-standard, no unnecessary overhead.

### Networking (CNI)
- **Current:** Flannel (bundled with K3s)
- **Usage:** Pod-to-pod networking across laptops
- **Assessment:** **Adequate — the Flannel multicast issue is irrelevant.** Since each pod is a fully isolated ROS2 island (all DDS traffic on localhost), Flannel only carries HTTP, WebSocket, and WebRTC traffic. These are all unicast and work perfectly over VXLAN. No CNI changes needed.

### Ingress
- **Current:** Traefik (bundled with K3s)
- **Usage:** Routes browser traffic to services based on URL path
- **Assessment:** Excellent choice. 64k stars, v3.7.7 (July 2026), first-class Kubernetes Ingress support, auto-discovers services.

### GPU Management
- **Current:** NVIDIA GPU Operator
- **Usage:** Time-slicing consumer GeForce GPUs across pods
- **Assessment:** Good fit. v26.3.3 (June 2026), 2.8k stars. Time-slicing on GeForce GPUs is supported and well-documented. Note: time-slicing provides soft partitioning only — no memory isolation or fault containment.

### Robotics Core
- **Current:** ROS2 Humble (planned)
- **Usage:** Middleware for robotics communication and node orchestration
- **Assessment:** Humble (May 2022) was LTS through 2027. Jazzy (May 2024) is now the current LTS through 2029. Starting a new project on Humble in 2026 is suboptimal.

### Simulation
- **Current:** Gazebo (via gzweb, planned)
- **Usage:** Physics simulation, 3D rendering in browser
- **Assessment:** The plan referenced `gzweb` (legacy Gazebo Classic). The modern approach is `gz-sim` Harmonic with its built-in `websocket_server` system plugin + the gzweb npm library for Three.js/WebGL rendering in the browser. No separate gzweb container is needed — the plugin runs inside the gz-sim process and serves scene data over a WebSocket.

### Video Streaming
- **Current:** Selkies (planned)
- **Usage:** WebRTC-based low-latency simulation streaming
- **Assessment:** **Excellent fit — keep it.** The plan misspelled the name as "Silkies" but the real project is **Selkies** (github.com/selkies-project/selkies): 1.9k stars, open-source low-latency WebRTC HTML5 remote desktop streaming, designed for Kubernetes with a native K8s operator, native NVIDIA GPU acceleration via OpenGL EGL/GLX and Vulkan, updated as recently as hours ago. This is a strong choice — WebRTC provides lower latency than VNC-based alternatives like noVNC.

### Code Editor
- **Current:** code-server (planned; replaces the earlier standalone Monaco Editor idea)
- **Usage:** Browser-based code editing for ROS2 node authoring, with an integrated terminal
- **Assessment:** Excellent choice. code-server is full VS Code in the browser — same mature editor core as Monaco, plus file explorer, extensions, and a built-in terminal for running `ros2 launch` directly from the browser. It runs as one supervised service inside the workspace image (port 7682), which removes the need to hand-build a Monaco frontend.

### Auto-Discovery / Cluster Join
- **Current:** Custom bash script (`join-cluster.sh`)
- **Usage:** Hardware profiling and node registration
- **Assessment:** Adequate for MVP but fragile. No error handling for partial failures, no idempotency, no uninstall/cleanup beyond `k3s-uninstall.sh`.

---

## Recommendations

### 1. Fix Selkies Spelling and Add GPU Configuration — HIGH (was CRITICAL, now HIGH since the component is real)

**Current:** "Silkies" (misspelled name in plan.md)
**Recommendation:** Keep Selkies — it is the correct, maintained project. Fix the spelling to "Selkies" in all manifests. Add GPU acceleration configuration via the `SELKIES_ENCODER` and `SELKIES_GPU_VENDOR` env vars for NVIDIA time-sliced GPUs.
**Rationale:** The plan's "Silkies" was a misspelling. The actual project is **Selkies** (selkies-project/selkies) — an open-source low-latency WebRTC HTML5 remote desktop streaming platform with 1.9k stars on GitHub, updated as recently as hours ago (July 2026). It is designed for Kubernetes (includes a K8s operator), supports native NVIDIA GPU acceleration via OpenGL EGL/GLX and Vulkan, and provides WebRTC transport which delivers significantly lower latency than VNC-based alternatives. This is a better fit for the Gazebo simulation streaming use case than noVNC because: (1) WebRTC adapts to network conditions, (2) GPU passthrough is first-class, (3) K8s operator matches the cluster architecture. No replacement is needed — just use the correct spelling and properly configure GPU encoding.

**Trade-offs:**

| Gain | Lose |
|---|---|
| Low-latency WebRTC transport (better than VNC) | Slightly more complex configuration than noVNC |
| Native NVIDIA GPU acceleration (EGL/GLX, Vulkan) | GPU encoding adds ~5% GPU overhead per stream |
| K8s operator for lifecycle management | WebRTC requires TURN/STUN for some network topologies |
| Actively maintained (updated hours ago) | |
| Matches the plan's architecture exactly | |

**Migration effort:** Low — correct the image name from any misspelled "silkies" reference to `selkies-project/selkies` or one of the prebuilt Docker images (`selkies-project/docker-selkies-egl-desktop` for the GPU-enabled variant). Set environment variables for GPU acceleration:
```yaml
env:
- name: SELKIES_ENCODER
  value: "nvidia_enc"
- name: SELKIES_GPU_VENDOR
  value: "nvidia"
```

**Evidence:**
- https://github.com/selkies-project/selkies — 1.9k stars, updated July 2026, low-latency WebRTC streaming for K8s
- https://github.com/selkies-project/docker-selkies-egl-desktop — 343 stars, prebuilt GPU-enabled Docker image with NVIDIA/EGL support
- https://github.com/selkies-project/selkies-operator — 73 stars, K8s operator for managing Selkies sessions

---

### 2. Upgrade ROS2 from Humble to Jazzy — HIGH

**Current:** ROS2 Humble (planned)
**Recommendation:** Use ROS2 Jazzy Jalisco (May 2024 LTS) instead
**Rationale:** Humble's LTS support ends ~2027. Jazzy is the current LTS (supported through 2029), bringing newer DDS features, improved Zero-Copy / Loaned Messages support, better QoS configuration, and more active community. Since the project is still in planning phase, the migration cost is zero — you'd be choosing the right base image from the start.

**Trade-offs:**

| Gain | Lose |
|---|---|
| LTS support through 2029 (2 extra years vs Humble) | Jazzy requires Ubuntu 24.04 Noble (vs Humble's 22.04) |
| Improved DDS performance (Fast DDS 2.14+) | Some older ROS2 packages may not yet have Jazzy binaries |
| Zero-Copy / Loaned Messages (reduces CPU in simulation) | |

**Migration effort:** Low (at design stage) — change base image from `osrf/ros:humble-desktop` to `osrf/ros:jazzy-desktop`. If Ubuntu 24.04 host compatibility is a constraint, evaluate further.
**Evidence:**
- https://github.com/ros2/ros2 — Latest release "Lyrical Luth" June 2026, Jazzy is current LTS
- https://ros.org/reps/rep-2000.html — ROS2 distribution lifecycle

---

### 3. Flannel is Fine — DDS Never Leaves the Pod — RESOLVED

**Current:** Flannel (bundled with K3s), single monolith pod per user
**Recommendation:** **Keep everything as-is.** Do not change Flannel. Do not add `hostNetwork`. Do not configure Fast DDS TCP transport. The single-pod-per-user architecture is correct — but for **isolation**, not for working around Flannel.
**Rationale:** The original audit suggested splitting pods and using `hostNetwork` / Fast DDS TCP to enable cross-pod DDS discovery. After further discussion, the project's actual requirement is **complete user isolation** — no user's ROS2 topics should ever be visible to another user. This is achieved by:
1. **Pod network namespace isolation** — each pod has its own network stack. DDS multicast on localhost never reaches another pod, regardless of Flannel.
2. **`ROS_DOMAIN_ID` per pod** — derived from StatefulSet ordinal (100, 101, 102...). Even if a DDS packet somehow escaped the pod, different UDP port ranges prevent cross-discovery.
3. **Single pod per user, single container per pod** — the workspace image runs code-server, Selkies, and gazebo-web under supervisord on shared localhost. No DDS traffic ever traverses Flannel.

Flannel only carries HTTP (code-server editor), WebSocket (gzweb scene data), and WebRTC (Selkies video) — all unicast, all well-supported.

**Trade-offs:**

| Gain | Lose |
|---|---|
| Zero network configuration — no env vars, no XML profiles, no hostNetwork | Cannot distribute containers across different hosts (all bound to one node) |
| Complete DDS isolation — impossible for user A to see user B's topics | Pod consumes entire laptop GPU slice even if user only uses the code editor (no sim) |
| Flannel works perfectly for what little traffic traverses it | |
| No Calico, no CNI swap, no extra complexity | |

**Migration effort:** None — this is the current architecture after the plan update.
**Evidence:**
- https://kubernetes.io/docs/concepts/services-networking/ — Kubernetes networking model: each pod has a unique IP, containers within share the network stack
- https://docs.ros.org/en/rolling/Tutorials/Advanced/ROS-Domain-ID.html — `ROS_DOMAIN_ID` isolates ROS2 computation graphs on port ranges

---

### 4. Add Observability Stack — MEDIUM

**Current:** None planned
**Recommendation:** Add lightweight observability: K3s Metrics Server (already bundled), Prometheus + Grafana on lightweight footprint, or just structured JSON logging with `fluent-bit`
**Rationale:** The platform runs arbitrary user ROS2 code. Without monitoring, a runaway simulation or memory leak in a user container can degrade the entire cluster. Laptop hardware is unreliable — you need to know when a node is overheating, running out of disk, or has a failing GPU. Lightweight Prometheus + Node Exporter + Grafana on the master node adds ~1GB RAM and provides substantial visibility.

**Trade-offs:**

| Gain | Lose |
|---|---|
| Detect resource exhaustion before pod eviction | Additional ~1GB RAM + ~10GB storage on master node |
| Track GPU time-slicing utilization | Setup complexity for non-K8s-expert operators |
| Alert on node health changes | Grafana dashboard needs maintenance |
| Historical data for capacity planning | |

**Migration effort:** Medium — deploy `kube-prometheus-stack` Helm chart or use K3s's built-in metrics server for basic pod metrics.
**Evidence:**
- https://github.com/prometheus-community/helm-charts — Prometheus stack for K8s
- https://docs.k3s.io/observability — K3s observability docs

---

### 5. Replace Custom Bash Discovery with a Structured Approach — MEDIUM

**Current:** `join-cluster.sh` (bash script)
**Recommendation:** Use a ConfigMap-driven node enrollment or Ansible playbook for node registration
**Rationale:** The current bash script is fragile — it has no retry logic, no validation of successful node join, no error reporting, and no idempotent rejoin. If a laptop re-joins with different hardware, it may not update node labels. A simple Ansible playbook or even a Python script with proper error handling would be more maintainable.

**Trade-offs:**

| Gain | Lose |
|---|---|
| Idempotent joins, proper error reporting | Drops "one-curl" simplicity |
| Can auto-taint nodes leaving abruptly | Requires Python or Ansible on host laptops |
| Better logging for troubleshooting | |
| Can integrate with a simple control dashboard | |

**Migration effort:** Low — rewrite `join-cluster.sh` in Python with `subprocess` calls to the K3s agent installer, wrapped with retry/validation logic.

---

### 6. Add Authentication Layer — MEDIUM

**Current:** None (plain HTTP access assumed)
**Recommendation:** Add at minimum a reverse-proxy basic auth or OAuth2-proxy in front of the platform services (code-server, Gzweb/Sim, Selkies/Streaming). The planned **Go control panel** (plan.md Phase 5) takes this further with JWT-issued sessions gating all workspace access.
**Rationale:** The platform allows writing and executing arbitrary code against a ROS2 environment with GPU access. On a shared LAN, any user who knows the URL can access it. Even for educational settings, basic credential protection prevents accidental resource consumption and provides a barrier to casual misuse.

**Trade-offs:**

| Gain | Lose |
|---|---|
| Prevent unauthorized access | Added login step for users |
| Audit trail of who ran what | Password management overhead |
| Rate limiting against resource abuse | |

**Migration effort:** Low — use Traefik's built-in BasicAuth middleware or deploy `oauth2-proxy` sidecar.
**Evidence:**
- https://doc.traefik.io/traefik/middlewares/http/basicauth/ — Traefik BasicAuth middleware

---

### 7. Update Gazebo Target — MEDIUM

**Current:** gzweb + Gazebo (version unspecified)
**Recommendation:** Target the modern `gz-sim` family with its `websocket_server` system plugin for web-based visualization. Do NOT use the legacy `gzweb`. Specifically: if you adopt ROS2 Jazzy, use **Gazebo Harmonic** (LTS, EOL May 2029); if you stay on Humble you can use Gazebo Fortress (LTS, EOL May 2027) or Harmonic. Avoid Gazebo Jetty (newest but not the LTS paired with ROS2).
**Rationale:** The plan references `gzweb`, which was a web client for the older Gazebo Classic / Ignition ecosystem. The current Gazebo Sim (`gz-sim`) ships a `websocket_server` system plugin (verified at `gazebosim/gz-sim/src/systems/websocket_server`) that serves 3D rendering to a web browser natively — no legacy `gzweb` needed. Per the official Gazebo install matrix: Gazebo Harmonic (LTS through 2029) is paired with both ROS2 Humble (on Ubuntu 22.04) and ROS2 Jazzy (on Ubuntu 24.04). Starting on Jetty (the newest) is not recommended for ROS2 because it lacks the LTS ROS2 pairing and there is no published `ros_gz` bridge version for it yet.

**Trade-offs:**

| Gain | Lose |
|---|---|
| Officially supported web visualization path via `websocket_server` plugin | Different API from gzweb — needs small migration work |
| Active development (1.4k stars, 7,500+ commits) | May need to build custom Docker image |
| Harmonic is LTS through 2029 (matches ROS2 Jazzy/Humble timelines) | |
| Better ROS2 integration via `ros_gz` bridge | |

**Migration effort:** Low — use a `gz-sim` Docker image (Harmonic or Fortress variant corresponding to your ROS2 distro) with the `websocket_server` plugin enabled in your SDF, instead of a custom gzweb image.
**Evidence:**
- https://github.com/gazebosim/gz-sim — 1.4k stars, latest release v10 "Jetty"
- https://github.com/gazebosim/gz-sim/tree/main/src/systems/websocket_server — verified existence of the `websocket_server` system plugin
- https://gazebosim.org/docs/latest/getstarted/ — official version/platform matrix (Harmonic LTS = 2029 EOL, paired with Humble/Jazzy; Jetty = newest, no ROS2 LTS pairing yet)

---



## Overkill & Simplification Opportunities

### Single Pod with All Containers — CORRECT (not overkill)

- **What it is:** Keeping ROS2+Gazebo, gzweb web app, Selkies, and Monaco in one pod
- **Why it's correct:** This is the **intentional architecture**, not a workaround. Each pod is a fully isolated ROS2 island — no cross-user topic leakage is possible by design. DDS stays on localhost. There is no valid reason to split the pod because ROS2 DDS communication **must** happen between simulation and other tools (e.g., a user's ROS2 node publishing to their own Gazebo topics), and no pod should ever talk DDS to another user's pod.
- **What not to do:** Do NOT split into separate sim and web UI pods. That would either require cross-pod DDS (breaking isolation) or force everything through a bridge layer (unnecessary complexity).
- **Resource caveat:** A user browsing Monaco without running a simulation still consumes a full GPU slice. This is acceptable for <50 users but worth monitoring.

### Custom Bash Join Script — SIMPLIFY

- **What it is:** `join-cluster.sh` with hardware detection and curl-to-K3s-agent
- **Why it's overkill (or rather, insufficient):** Bash is not the right tool for error handling, idempotency, and logging at the complexity level of this script. The script is only ~20 lines but handles hardware profiling, conditional logic, and cluster enrollment.
- **What to do instead:** A 40-line Python script using `subprocess` with proper `try/except`, `logging`, and `retry` logic would be more robust and easier to debug.
- **Complexity saved:** Support time. When a laptop fails to join, structured error messages reduce debugging from "run the bash script with -x" to "check the log file."

---

## Architecture & System Design Assessment

### What's working well
- **K3s as the orchestration backbone** is the correct choice for this use case — lightweight, edge-optimized, bundles the right components
- **Single-entry-point via Traefik Ingress** on the master node simplifies guest access — one URL routes to all services
- **GPU time-slicing** correctly addresses the constraint that consumer GPUs don't support hardware MIG partitioning
- **Pod co-location of DDS-dependent containers** is the correct isolation model — DDS stays on localhost, no cross-user topic leakage
- **Dynamic node join/leave** model is well-thought-out: the description of heartbeat-based eviction is correct

### What's missing
- **No authentication/authorization** — anyone on the LAN can run code on the cluster
- **No pod resource quota or limit ranges** — a single user could request all 4 GPU time-slices, starving others
- **No persistent storage strategy** — user code and simulation state disappears when pods restart
- **No health checks or readiness probes** — pods in a crash loop remain in service
- **No pod priority or preemption** — no mechanism to guarantee that critical infrastructure pods (Traefik, DNS) outcompete user simulation pods during resource contention
- **No network policy** — user pods can reach each other via HTTP/WebSocket (not DDS — that's localhost-isolated). Consider Calico or Cilium for NetworkPolicy if this is a concern.
- **No CI/CD pipeline** — no automated build of the custom Docker images (Monaco, streaming, etc.)

### What's over-engineered
- **K3s for <50 users on a single LAN** might be swapped for a simpler docker-compose or Nomad setup, but K3s is justified given the dynamic node join/leave, GPU scheduling, and self-healing requirements. Keep it.

### Suggested target architecture

```
                     ┌──────────────────────────────────┐
                     │      Master Laptop (LAN)          │
                     │   ┌──────────────────────────┐   │
Guest Browser ──────►   │  Traefik Ingress           │   │
                     │   │  /code /sim /stream       │   │
                     │   │  + OAuth2-proxy (planned) │   │
                     │   └──────────────────────────┘   │
                     │                                   │
                     │   ┌──────────────────────────┐   │
                     │   │  Monitoring (Prometheus)  │   │
                     │   │  + Node Exporter          │   │
                     │   └──────────────────────────┘   │
                     └──────────┬───────────────────────┘
                                │
                   ┌────────────┴────────────┐
                   │  Flannel CNI             │
                   │  (HTTP / WS / WebRTC     │
                   │   only — no DDS)         │
                   └────────────┬────────────┘
                                │
                ┌───────────────┴───────────────────┐
                │                                   │
   ┌────────────┴────────────┐      ┌──────────────┴───────────┐
   │   Host Laptop 1         │      │   Host Laptop 2          │
   │                         │      │                          │
   │ ┌─── Pod A (user1) ──┐ │      │ ┌─── Pod C (user3) ──┐  │
   │ │ ros2-gazebo        │ │      │ │ ros2-gazebo        │  │
   │ │   domain=101       │ │      │ │   domain=103       │  │
   │ │ sim-web-app :8080   │ │      │ │ sim-web-app :8080   │  │
   │ │ selkies     :8081   │ │      │ │ selkies     :8081   │  │
   │ │ monaco      :3000   │ │      │ │ monaco      :3000   │  │
   │ │ DDS: localhost only │ │      │ │ DDS: localhost only │  │
   │ └─────────────────────┘ │      │ └─────────────────────┘  │
   │                         │      │                          │
   │ ┌─── Pod B (user2) ──┐ │      │                          │
   │ │ ros2-gazebo        │ │      │                          │
   │ │   domain=102       │ │      │                          │
   │ │ sim-web-app :8080   │ │      │                          │
   │ │ selkies     :8081   │ │      │                          │
   │ │ monaco      :3000   │ │      │                          │
   │ │ DDS: localhost only │ │      │                          │
   │ └─────────────────────┘ │      │                          │
   └─────────────────────────┘      └──────────────────────────┘
```

Key architecture decisions:
1. **Isolation-first** — each pod is a self-contained ROS2 island. DDS stays on localhost.
2. **StatefulSet with ROS_DOMAIN_ID** — ordinal-based (100, 101, 102...) prevents cross-pod topic leakage.
3. **Flannel is irrelevant** — carries only HTTP/WS/WebRTC (unicast, works perfectly).
4. **No hostNetwork, no Fast DDS TCP** — not needed when DDS never leaves localhost.
5. **Selkies** (not Silkies) — correctly named, GPU-configured, for desktop streaming.
6. **Gazebo Sim websocket_server plugin** — replaces legacy gzweb container.

---

## Dependency Health Summary

| Component | Version (Planned) | Last Release | Status | Action |
|---|---|---|---|---|
| K3s | latest | v1.36.2+k3s1 (Jun 2026) | Active (CNCF) | Keep |
| Flannel | bundled | v0.28.7 (Jul 2026) | Active | Keep — DDS stays on localhost, Flannel carries only unicast HTTP/WS/WebRTC |
| NVIDIA GPU Operator | latest | v26.3.3 (Jun 2026) | Active | Keep |
| Traefik | bundled | v3.7.7 (Jul 2026) | Active (CNCF) | Keep |
| ROS2 Humble | humble | EOL ~2027 | Aging | Upgrade to Jazzy (LTS through 2029) |
| Gazebo (gz-sim) | gz-sim Jetty | v10 "Jetty" (Oct 2025) | Active | Use Gazebo Harmonic (LTS through 2029) — paired with both Humble and Jazzy |
| gzweb | referenced | N/A | Legacy / superseded | Use gz-sim's `websocket_server` system plugin |
| Selkies | planned | Updated Jul 13, 2026 | Active (1.9k stars, K8s-native WebRTC) | Keep — fix spelling, add GPU config |
| Monaco Editor | planned | N/A | Active | Keep |
| CoreDNS | bundled | N/A | Active (CNCF) | Keep |
| Containerd | bundled | N/A | Active (CNCF) | Keep |

---

## Action Plan (Prioritized)

### Immediate (before implementation)
- [ ] HIGH: Fix the "Silkies" → "Selkies" spelling in all manifests. Add GPU encoder configuration (SELKIES_ENCODER, SELKIES_GPU_VENDOR) for NVIDIA time-sliced GPUs.
- [ ] HIGH: Decide on ROS2 distribution — pick Jazzy instead of Humble before writing any manifests.
- [ ] HIGH: Set `ROS_DOMAIN_ID` per pod — use StatefulSet ordinal (100 + index) for deterministic unique IDs. This ensures zero cross-pod topic leakage.
- [ ] MEDIUM: Pick the matching Gazebo release — Gazebo Harmonic (LTS through 2029) pairs with both ROS2 Humble and Jazzy.
- [ ] MEDIUM: Replace legacy gzweb container with Gazebo Sim `websocket_server` plugin + lightweight gzweb npm web app.

### Short-term (MVP implementation)
- [ ] HIGH: Write CI/CD pipeline (GitHub Actions) to build and push custom Docker images (Monaco web UI, streaming container, Gazebo with websocket plugin).
- [ ] MEDIUM: Add authentication via Traefik BasicAuth or OAuth2-proxy.
- [ ] MEDIUM: Add resource quotas and limit ranges per namespace (separate "users" and "system" namespaces).
- [ ] MEDIUM: Rewrite `join-cluster.sh` as a Python script with proper error handling and logging.

### Medium-term (next quarter)
- [ ] MEDIUM: Deploy lightweight monitoring (Prometheus + Node Exporter + Grafana) on master node.
- [ ] MEDIUM: Implement network policies to isolate user simulation pods from each other (Calico is a valid option here for policy, NOT multicast).
- [ ] MEDIUM: Add persistent storage (via K3s's local-path-provisioner or Longhorn) for user code persistence.


### Cleanup (when convenient)
- [ ] LOW: Document the exact Gazebo version and websocket plugin configuration being used.
- [ ] LOW: Create a troubleshooting guide for common cluster-join failures.

---

## Sources

### K3s
- https://github.com/k3s-io/k3s — 33.5k stars, v1.36.2+k3s1, CNCF project, active maintenance
- https://k3s.io/ — Official site, lightweight K8s for edge/IoT

### Flannel
- https://github.com/flannel-io/flannel — 9.5k stars, v0.28.7, active

### NVIDIA GPU Operator
- https://github.com/NVIDIA/gpu-operator — 2.8k stars, v26.3.3, official NVIDIA project

### Traefik
- https://github.com/traefik/traefik — 64k stars, v3.7.7, active

### ROS2
- https://github.com/ros2/ros2 — 5.8k stars, latest Lyrical Luth release
- https://github.com/ros2/rmw_fastrtps — Fast DDS middleware for ROS2, QoS XML configuration docs

### Gazebo
- https://github.com/gazebosim/gz-sim — 1.4k stars, gz-sim v10 "Jetty"

### noVNC (Fallback VNC-based alternative)
- https://github.com/novnc/noVNC — 13.8k stars, v1.7.0 (April 2026), HTML5 VNC client
- https://github.com/novnc/websockify — WebSocket-to-TCP proxy companion

### Calico (CNI Alternative — for NetworkPolicy, NOT multicast)
- https://github.com/projectcalico/calico — 7.3k stars, v3.32.1, eBPF support
- https://docs.tigera.io/calico/latest/networking/configuring/vxlan-ipip — Calico overlay docs (confirms VXLAN/IP-in-IP are unicast tunnels; multicast is not supported by overlay CNI)

### Other Sources
- https://docs.k3s.io/networking/cni — K3s CNI options documentation
- https://fast-dds.docs.eprosima.com/ — eProsima Fast DDS documentation
- https://docs.ros.org/en/rolling/Concepts/About-Quality-of-Service-Settings.html — ROS2 QoS settings
- https://design.ros2.org/articles/zero_copy.html — ROS2 zero-copy / loaned messages
- https://github.com/selkies-project/selkies — Verified Selkies project: 1.9k stars, active WebRTC streaming platform for K8s
