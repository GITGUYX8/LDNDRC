# LOCAL DYNAMIC NODE DISCOVERY ROS CLUSTER

## 1. Project Overview
This platform is a local-first, browser-accessible robotics simulation environment built on a dynamic Kubernetes (K3s) cluster. It is designed to run entirely on a network of consumer laptops without a dedicated server.

High-end laptops ("Hosts") contribute their CPU, RAM, and GPU resources to run ROS2 and Gazebo simulations. Low-end laptops ("Guests") act as thin clients, accessing the platform via a web browser to write code (Monaco), view 3D simulation (Gazebo Sim web app), and stream desktop (Selkies).

### Key Characteristics
*   **Workload per Pod:** 3-4 CPU cores, 4-8GB RAM, 1 GPU (time-sliced).
*   **Hardware Target:** Consumer gaming/workstation laptops (NVIDIA GeForce GPUs).
*   **Networking:** Local LAN (Wi-Fi/Ethernet), no cloud dependencies.
*   **Isolation:** Each user gets a fully isolated ROS2 environment — no topic cross-talk between users.

---

## 2. Tech Stack

| Layer                  | Technology                         | Purpose                                                                  |
| :--------------------- | :--------------------------------- | :----------------------------------------------------------------------- |
| **Orchestration**      | K3s (Lightweight Kubernetes)       | Manages cluster state, scheduling, and pod lifecycle on laptops.         |
| **Container Runtime**  | Containerd                         | Lightweight runtime bundled with K3s.                                    |
| **Networking (CNI)**   | Flannel                            | Pod-to-pod networking across laptops. Irrelevant for DDS — all ROS2 traffic stays on pod localhost. |
| **Ingress Controller** | Traefik (Bundled in K3s)           | Routes browser traffic to correct pods based on URL path.                |
| **GPU Management**     | NVIDIA GPU Operator                | Automates GPU driver configuration and time-slicing on consumer laptops. |
| **Auto-Discovery**     | Bash Scripting                     | Scans laptop hardware (CPU/RAM/GPU) and triggers cluster join.           |
| **Robotics Core**      | ROS2 (Jazzy LTS recommended)       | Middleware for robotics communication.                                   |
| **Simulation**         | Gazebo Sim Harmonic (LTS 2029)     | Physics simulation with `websocket_server` plugin for web rendering.     |
| **3D Web View**        | gzweb (npm library)                | Three.js-based WebGL client — renders simulation in browser.             |
| **Desktop Streaming**  | Selkies                            | WebRTC-based low-latency streaming for interactive desktop (terminal + code). |
| **Code Editor**        | code-server                       | VS Code in the browser, integrated terminal (replaces standalone Monaco). |
| **Control Panel (backend)** | Go (client-go, JWT, net/http) | Sessions, auth, pod provisioning, WebSocket proxy — single static binary. |

---

## 3. Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Guest Laptop (Browser)                   │
│  http://ros-platform.local/code   → Monaco editor           │
│  http://ros-platform.local/sim    → 3D sim view (WebGL)     │
│  http://ros-platform.local/stream → Selkies desktop stream  │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│                 Master Laptop (K3s + Traefik)                │
│                                                              │
│  Traefik Ingress ──► Service (session-affinity to pods)     │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│              Host Laptop (GPU time-sliced x4)                │
│                                                              │
│  ┌────────── Pod A (user1) ────────────────────┐             │
│  │ ros2-gazebo   │ ROS_DOMAIN_ID=101           │             │
│  │   (sim + websocket_server plugin)           │             │
│  │ sim-web-app  │ port 8080 (Three.js gzweb)   │             │
│  │ selkies      │ port 8081 (WebRTC desktop)   │             │
│  │ monaco-editor│ port 3000 (code)             │             │
│  │                                              │             │
│  │ DDS discovery: localhost only                │             │
│  │ No cross-pod ROS2 visibility                 │             │
│  └──────────────────────────────────────────────┘             │
│                                                              │
│  ┌────────── Pod B (user2) ────────────────────┐             │
│  │ ros2-gazebo   │ ROS_DOMAIN_ID=102           │             │
│  │ sim-web-app  │ port 8080                    │             │
│  │ selkies      │ port 8081                    │             │
│  │ monaco-editor│ port 3000                    │             │
│  │                                              │             │
│  │ Fully isolated — no topic leakage to Pod A  │             │
│  └──────────────────────────────────────────────┘             │
│                                                              │
│  Flannel only routes HTTP / WebSocket / WebRTC traffic       │
│  DDS never leaves localhost — Flannel multicast irrelevant   │
└─────────────────────────────────────────────────────────────┘
```

### Isolation Model

Each pod is a **fully isolated ROS2 environment**:

- All 4 containers share the pod network namespace — DDS discovery/multicast stays on `localhost`
- `ROS_DOMAIN_ID` is unique per pod (derived from StatefulSet ordinal)
- Even if two pods land on the same host, their DDS traffic cannot interfere because they're on different virtual Ethernet pairs
- The cluster provides only: GPU scheduling (which laptop runs the pod), web routing (Traefik), and resource guarantees

```
Pod A (localhost, domain=101): topic /cmd_vel ──► subscriber inside same pod
Pod B (localhost, domain=102): topic /cmd_vel ──► subscriber inside same pod
                                          ↑ Localhost isolation means
                                            zero cross-pod discovery,
                                            even with same domain_id.
                                            domain_id adds defense-in-depth.
```

---

## 4. Implementation Plan

### Phase 1: Infrastructure Setup & Auto-Discovery (MVP)
**Goal:** Establish the K3s cluster and allow laptops to dynamically join based on their hardware.

**1. Master Node Setup:**
Choose a laptop to be the Master (must have a static LAN IP, e.g., `192.168.1.50`). Disable sleep mode.
```bash
curl -sfL https://get.k3s.io | sh -
cat /var/lib/rancher/k3s/server/node-token
```

**2. Auto-Discovery Agent (`join-cluster.sh`):**
See `join-cluster.sh` at the project root — it accepts env var overrides for testing:

```bash
# Production usage (detects real hardware):
export MASTER_IP=192.168.1.50
export NODE_TOKEN=$(cat /var/lib/rancher/k3s/server/node-token)
./join-cluster.sh
```

The script supports `TEST_CPU`, `TEST_RAM` (GB), and `TEST_GPU` (nvidia|none) env vars to override hardware detection for testing without dedicated hardware:

```bash
# Simulate a Host laptop (12 cores, 16GB, NVIDIA GPU):
TEST_CPU=12 TEST_RAM=16 TEST_GPU=nvidia ./join-cluster.sh
# → "High-end laptop detected. Joining as HOST."

# Simulate a Guest laptop (4 cores, 4GB, no GPU):
TEST_CPU=4 TEST_RAM=4 TEST_GPU=none ./join-cluster.sh
# → "Low-end laptop detected. Use browser to access platform."
```

**Testing (single-laptop):** Run `./test-join.sh` — no cluster, no Docker, no VMs needed. It tests all 4 hardware detection branches using env var overrides in ~2 seconds.

---

### Phase 2: GPU Time-Slicing Configuration
**Goal:** Allow multiple ROS2/Gazebo pods to share a single consumer laptop GPU. Consumer GPUs (GeForce) do not support hardware MIG, so we use software time-slicing.

**1. Install NVIDIA GPU Operator:**
```bash
helm repo add nvidia https://nvidia.github.io/gpu-operator
helm install gpu-operator nvidia/gpu-operator -n gpu-operator --create-namespace
```

**2. Configure Time-Slicing ConfigMap:**
```yaml
# gpu-time-slicing-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: time-slicing-config
  namespace: gpu-operator-resources
data:
  default: |-
    version: v1
    sharing:
      timeSlicing:
        resources:
          - name: nvidia.com/gpu
            replicas: 4
```

**3. Patch the ClusterPolicy:**
```bash
kubectl apply -f gpu-time-slicing-config.yaml
kubectl patch clusterpolicy cluster-policy -n gpu-operator --type='json' \
  -p='[{"op": "add", "path": "/spec/devicePlugin/config", "value": {"name": "time-slicing-config", "default": "default"}}]'
```

---

### Phase 3: ROS2, Gazebo, & Web Integration
**Goal:** Deploy isolated per-user pods with the full ROS2/Gazebo/web stack.

#### Design Decisions
- **StatefulSet** — ordinal index (0, 1, 2...) provides deterministic `ROS_DOMAIN_ID` per pod
- **Single pod per user** — all containers share one pod network namespace; DDS traffic stays on `localhost`
- **`websocket_server` plugin** — runs inside `gz-sim` process, no separate Gzweb server needed
- **gzweb npm library** — served by a lightweight web app container for the Three.js/WebGL 3D view
- **Selkies** — streams the interactive desktop (terminal + Monaco workspace) via WebRTC

**1. Pod Manifest (`ros2-platform.yaml`):**
```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: ros2-platform
spec:
  serviceName: ros2-platform-svc
  replicas: 3
  selector:
    matchLabels:
      app: ros2-platform
  template:
    metadata:
      labels:
        app: ros2-platform
    spec:
      nodeSelector:
        node-role.kubernetes.io/role: host
      containers:
        # 1. ROS2 + Gazebo Sim (headless, with websocket_server plugin)
        - name: ros2-gazebo
          # Build a custom image: ROS2 Jazzy + Gazebo Sim Harmonic
          # with websocket_server plugin enabled in the world SDF.
          image: your-registry/ros2-gazebo-sim:latest
          env:
            - name: POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
          resources:
            requests:
              cpu: "3"
              memory: "4Gi"
              nvidia.com/gpu: "1"
            limits:
              cpu: "4"
              memory: "8Gi"
              nvidia.com/gpu: "1"
          command: ["/bin/bash", "-c"]
          args:
            - |
              ORDINAL="${POD_NAME##*-}"
              export ROS_DOMAIN_ID=$((100 + ORDINAL))
              ros2 launch turtlebot3_gazebo empty_world.launch.py &
              gz launch --verbose /etc/gz-sim/websocket.gzlaunch &
              sleep infinity

        # 2. Sim Web App (serves gzweb npm Three.js viewer, proxies WebSocket to gz-sim)
        - name: sim-web-app
          image: nginx:alpine
          ports:
            - containerPort: 8080

        # 3. Selkies (Desktop streaming via WebRTC)
        - name: selkies
          image: selkies-project/docker-selkies-egl-desktop:latest
          env:
            - name: SELKIES_ENCODER
              value: "nvh264enc"
            - name: SELKIES_GPU_VENDOR
              value: "nvidia"
          ports:
            - containerPort: 8081

        # 4. Monaco Editor (Code interface)
        - name: monaco-editor
          image: node:18-alpine
          ports:
            - containerPort: 3000
```

**2. ROS_DOMAIN_ID Mapping:**
The StatefulSet creates pods named `ros2-platform-0`, `ros2-platform-1`, etc. The init script extracts the ordinal:

| Pod | Ordinal | ROS_DOMAIN_ID | Isolated from |
|---|---|---|---|
| ros2-platform-0 | 0 | 100 | All other pods |
| ros2-platform-1 | 1 | 101 | All other pods |
| ros2-platform-2 | 2 | 102 | All other pods |

**3. Gazebo Sim `websocket_server` Plugin Config (`websocket.gzlaunch`):**
```xml
<?xml version='1.0'?>
<gz version='1.0'>
  <plugin name='gz::launch::WebsocketServer'
          filename='gz-launch-websocket-server'>
    <port>9002</port>
    <publication_hz>30</publication_hz>
    <max_connections>-1</max_connections>
  </plugin>
</gz>
```

Build this into the custom Docker image alongside ROS2 + Gazebo Sim Harmonic.

---

### Phase 4: Web Routing & Final Platform Access
**Goal:** Expose pods so Guest laptops can access them via browser. Use session affinity so a user stays on their assigned pod.

**1. Service (`ros2-service.yaml`):**
```yaml
apiVersion: v1
kind: Service
metadata:
  name: ros2-platform-svc
spec:
  sessionAffinity: ClientIP
  sessionAffinityConfig:
    clientIP:
      timeoutSeconds: 3600
  selector:
    app: ros2-platform
  ports:
    - name: web-monaco
      port: 3000
      targetPort: 3000
    - name: web-gzweb
      port: 8080
      targetPort: 8080
    - name: web-selkies
      port: 8081
      targetPort: 8081
```

**2. Ingress (`ros2-ingress.yaml`):**
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: platform-ingress
spec:
  rules:
    - host: ros-platform.local
      http:
        paths:
          - path: /code
            pathType: Prefix
            backend:
              service:
                name: ros2-platform-svc
                port:
                  number: 3000
          - path: /sim
            pathType: Prefix
            backend:
              service:
                name: ros2-platform-svc
                port:
                  number: 8080
          - path: /stream
            pathType: Prefix
            backend:
              service:
                name: ros2-platform-svc
                port:
                  number: 8081
```

---

### Phase 5: Go Control Panel (Backend)
**Goal:** A lightweight session-management API and gateway in Go, deployed as a single static binary. Reuses the reference control panel's behavior (pod CRUD, auth, WebSocket proxy) without the Node runtime.

**1. Module layout (`control-panel/`):**
```
control-panel/
  cmd/server/main.go     — HTTP server entrypoint
  internal/auth/         — JWT issue + verify
  internal/sessions/     — pod create/status/delete via client-go
  internal/gateway/      — httputil.ReverseProxy to session pods
  internal/httpapi/      — REST handlers, health, metrics
  web/                   — embedded control page (embed.FS)
```

**2. Key dependencies:**
```go
import (
  "k8s.io/client-go/kubernetes"
  "k8s.io/client-go/tools/clientcmd"
  "github.com/golang-jwt/jwt/v5"
)
```

**3. Endpoints:**
| Method | Path | Purpose |
|---|---|---|
| POST | `/api/sessions` | Provision a workspace pod |
| GET | `/api/sessions` | List sessions (status, node) |
| GET | `/api/sessions/{id}` | Session detail |
| DELETE | `/api/sessions/{id}` | Tear down a session |
| POST | `/api/auth/login` | Issue JWT |
| GET | `/healthz` | Liveness |
| WS | `/ws/{session}` | WebSocket proxy into a pod (code-server / gzweb / selkies) |

**4. Deployment:** static binary + minimal container (e.g. `gcr.io/distroless/static`) run as a Deployment, exposed via Traefik behind auth.

---

## 5. Operational Guidelines

### How Nodes Join and Exit
*   **Joining:** A user runs `./join-cluster.sh`. The script detects hardware, starts the K3s agent, and registers the laptop with the Master. The GPU operator automatically injects the time-slicing config into the new node.
*   **Exiting Gracefully:** User runs `/usr/local/bin/k3s-uninstall.sh`.
*   **Exiting Abruptly (Laptop closed/battery died):** The Master node loses heartbeat within ~40 seconds. The node is marked `NotReady`. Kubernetes automatically evicts the pods and reschedules them onto other available Host laptops.

### End User Experience
1.  **Host Users:** Turn on laptops, run the script, and leave them plugged in.
2.  **Guest Users:** Open browser (Chrome/Firefox) on any laptop.
3.  Navigate to `http://ros-platform.local/code` to access the code-server editor (VS Code in the browser).
4.  Write ROS2 code and hit "Run" — it executes inside the assigned pod, fully isolated from other users. Use the built-in terminal to run `ros2 launch` directly in the browser.
5.  Navigate to `http://ros-platform.local/sim` to view the Gazebo Sim 3D output via WebGL.
6.  Navigate to `http://ros-platform.local/stream` for the Selkies interactive desktop stream (terminal + code workspace).

### Control Panel (Go Backend)
The session-management backend is written in Go and deployed as a single static binary — no Node runtime. It reuses the behavior of the reference control panel but in Go:

| Backend need | Go solution |
|---|---|
| Create/delete session pods | `k8s.io/client-go` |
| Auth / session tokens | `github.com/golang-jwt/jwt` |
| WebSocket proxy to pods | `net/http/httputil.ReverseProxy` |
| Serving a minimal control page | `embed.FS` or a small htmx page |
| Metrics / health | `net/http` + Prometheus client |

### Security Model
- Each pod has a unique `ROS_DOMAIN_ID` — DDS traffic is port-isolated even if a packet leaked out of localhost.
- `ROS_AUTOMATIC_DISCOVERY_RANGE=LOCALHOST` pins ROS2 discovery to localhost as a second isolation layer.
- DDS runs over UDP multicast on localhost only — no DDS packet ever traverses the pod network or Flannel overlay.
- Flannel carries only HTTP (code-server), WebSocket (gzweb), and WebRTC (Selkies) traffic.
- Cross-pod ROS2 topic discovery is **impossible by design** — the network topology prevents it at the kernel level (different network namespaces).
- **Recommended:** The Go control panel terminates auth (JWT) before traffic reaches Traefik; add basic auth / OAuth2-proxy / mTLS in front before opening beyond LAN.
