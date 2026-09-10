# LDNDRC Roadmap — Done vs Remaining (Visual Guide)

> How to read this doc: **§1 is the map** (done = green, remaining = amber,
> blocked/heavy = red). **§2 explains every remaining item** in build order
> with acceptance criteria. **§3 is the suggested sequence** with effort
> estimates. Mermaid diagrams render on GitHub/GitLab; an ASCII fallback
> follows each one.

## 1. The Map

### 1a. Build journey (where we are)

```mermaid
flowchart LR
    classDef done fill:#1d5c2e,stroke:#3ecf8e,color:#fff
    classDef next fill:#6b4e12,stroke:#f5b86c,color:#fff
    classDef later fill:#3a3f4b,stroke:#8b93a7,color:#fff
    classDef heavy fill:#5c1d1d,stroke:#f56c6c,color:#fff

    C01["CP01<br/>join script + k3d tests"]:::done
    C02["CP02<br/>workspace images + isolation"]:::done
    C03["CP03<br/>Go control panel skeleton"]:::done
    C04["CP04<br/>manifests + UI/UX design"]:::done
    C05["CP05<br/>per-session K8s provisioning"]:::done
    C06["CP06<br/>control panel on K3s"]:::done
    C07["CP07<br/>cookie gateway + host routing"]:::done
    C08["CP08<br/>static verification"]:::done
    C09["CP09<br/>live K3s verification"]:::done
    C10["CP10<br/>Headlamp visuals"]:::done
    C11["CP11<br/>optional GPU"]:::done
    C12["CP12<br/>stand-in ready path"]:::done
    YOU["YOU ARE HERE"]:::next

    R1["R1<br/>remove legacy ingress"]:::next
    R2["R2<br/>session persistence"]:::next
    R3["R3<br/>CSRF protection"]:::next
    R4["R4<br/>student frontend UI"]:::later
    R5["R5<br/>host onboarding (ldndrc-join)"]:::later
    R6["R6<br/>real workspace image"]:::heavy
    R7["R7<br/>hardening: TLS, policy, obs."]:::later

    C01 --> C02 --> C03 --> C04 --> C05 --> C06 --> C07 --> C08 --> C09 --> C10 --> C11 --> C12 --> YOU --> R1 --> R2 --> R3 --> R4 --> R5 --> R6 --> R7
```

```text
ASCII fallback (CP = done checkpoint, Rn = remaining item):

CP01 -> CP02 -> CP03 -> CP04 -> CP05 -> CP06 -> CP07 -> CP08 -> CP09 -> CP10 -> CP11 -> CP12
                                                                                        |
                                                                                   YOU ARE HERE
                                                                                        |
                           R1 -> R2 -> R3 -> R4 -> R5 -> R6 (heavy, parallel) -> R7
```

### 1b. System map (what exists vs what is missing)

```mermaid
flowchart TB
    classDef done fill:#1d5c2e,stroke:#3ecf8e,color:#fff
    classDef missing fill:#6b4e12,stroke:#f5b86c,color:#fff
    classDef legacy fill:#5c1d1d,stroke:#f56c6c,color:#fff

    Browser(["Guest browser"]):::done
    DNS["*.ros-platform.local<br/>(hosts file / LAN DNS)"]:::missing
    Traefik["Traefik (k3d loadbalancer :80/:443)"]:::done
    CP["Control panel (Go)<br/>auth + sessions + gateway"]:::done
    Store["Session store<br/>IN-MEMORY - lost on restart"]:::missing
    K8s["K3s API (k3d)"]:::done
    Sess["Per-user Deployment+Service+PVC"]:::done
    WS["Workspace pod<br/>(demo-standin TODAY)"]:::missing
    RealWS["Real Jazzy/Harmonic image"]:::missing
    Head["Headlamp dashboard"]:::done
    OldIng["ros2-ingress.yaml<br/>REMOVED (checkpoint-13)"]:::done
    UI["Student frontend<br/>(login/launch/workspace)"]:::missing
    Join["ldndrc-join onboarding"]:::missing

    Browser --> DNS --> Traefik --> CP
    CP --> Store
    CP --> K8s --> Sess --> WS
    Sess -.-> RealWS
    Browser --> Head
    Traefik -.-> OldIng
    CP -.-> UI
    K8s -.-> Join
```

Legend:

| Color | Meaning |
|---|---|
| Green | Built, deployed, verified live |
| Amber | Missing or provisional — real remaining work |
| Red | Legacy — works but scheduled for removal (none left after R1) |

## 2. Remaining Work in Detail

### R1 — Remove legacy ingress (done in checkpoint-13)

`manifests/ros2-ingress.yaml` routed `/code`, `/stream`, `/sim` on the
bare hostname to the old shared `ros2-platform-svc`. Host-based routing
(CP07, proven live in CP09/CP12) replaced it, so the ingress, the shared
Service (`ros2-service.yaml`), and the fixed StatefulSet
(`ros2-platform.yaml`) were deleted. Verified: only the control-panel
ingress exists live, gateway hosts keep their proven 200/502/401 pattern,
and the old `/code` path 404s.

### R2 — Session persistence (medium, observed pain twice)

The store is in-memory: every control-panel restart wipes sessions and
orphans their K3s resources (seen live in CP09 and CP11, cleaned by hand).

- Persist sessions (JSON file like the onboarding design's node store, or SQLite/Postgres later) and reconcile on boot: adopt matching live resources, mark the rest for cleanup.
- Verify: restart control panel → sessions survive → gateway still routes → no orphans in `kubectl get`.
- Effort: half a day (file store) to days (DB + migration).

### R3 — CSRF protection (small–medium, reference parity)

The reference project requires a CSRF token on mutating browser requests;
our cookie API currently does not. Any site the student visits could POST to
our API with their cookie attached.

- Issue CSRF token at login, require `X-CSRF-Token` header on POST/DELETE session routes, keep bearer-token API path unchanged.
- Verify: unit tests (missing/invalid token → 401/403), live check via curl.
- Effort: half a day.

### R4 — Student frontend UI (large, design exists)

`uiux-design.md` specifies login → launch stepper → workspace shell
(Code/Sim/Split views) as a React+Vite SPA embedded in the Go binary via
`embed.FS`. Nothing is built yet; the API it needs is verified live.

- Scaffold `control-panel/web/`, wire JWT + cookie auth, session CRUD polling, iframe embedding of editor/desktop, gzweb component for sim.
- Verify phase-wise: UI-1 login+launch, UI-2 workspace shell, UI-3 admin view (matches the design doc's phasing).
- Effort: days to a week.

### R5 — Host onboarding `ldndrc-join` (large, design ready)

`onboarding-design-report.md` (amended 2026-09-06 for the in-cluster control
panel) specifies the zero-touch flow: mDNS discovery, nodes register/approve
API, short-lived bootstrap tokens via client-go, cross-platform join binary.
Today only the manual `join-cluster.sh` path exists. Approval dashboard UI
is deferred until the R4 frontend lands — P1 approval is API/`curl`-driven.

- Implement in the report's phase order — **P1 nodes API done
  (checkpoint-14), P2 advertise done (checkpoint-17)**; P4 join binary,
  P5–P6 next — keeping the bash path until the Go binary is proven on
  real hardware.
- Verify: a real second laptop joins as Host, appears Ready with the Host
  label, and schedules session pods; denial path tested too.
- Effort: several days; needs a second physical machine for the real proof.

### R6 — Real workspace image (heavy, needs hardware)

The `demo-standin` proves plumbing only. The true workspace (ROS2 Jazzy +
Gazebo Harmonic + code-server + Selkies, per `images/`) is a ~10GB+ build
and needs a GPU host to shine.

- Build `images/base` → `images/workspace` on a machine with disk and time;
  import into the cluster; flip `SESSION_IMAGE` back from the stand-in.
- Verify: session reaches `ready` with real services on 7682/8080/9002;
  gateway editor/desktop/sim all 200; TurtleBot3 sim streams.
- Effort: a day of build babysitting; full value only on GPU hardware.

### R7 — Production hardening (ongoing track)

TLS/mTLS on the control panel API, NetworkPolicies isolating session pods,
Prometheus/Grafana observability, log aggregation, resource quotas. Each is
independently shippable; start with TLS + quotas when leaving the LAN-demo
stage.

## 3. Suggested Sequence

| Step | Item | Why here | Effort |
|---|---|---|---|
| 1 | ~~R1 legacy ingress removal~~ done (checkpoint-13) | Cleanup complete | < 1h |
| 2 | R3 CSRF protection | Small, closes a real browser-threat hole | 0.5 day |
| 3 | R2 session persistence | Medium; stops the restart-orphan pain | 0.5–2 days |
| 4 | R4 student frontend | Large but fully unblocked by the verified API | days |
| 5 | R5 host onboarding | Needs a second physical laptop | days |
| 6 | R7 hardening | As you approach real users | ongoing |
| ∥ | R6 real image (parallel) | Heavy; run alongside whenever hardware/disk allow | 1+ day |

Guide for contributors: pick the topmost unfinished row, read its section in
§2, implement, verify with the listed checks, then record a checkpoint report
in `local_dev_docs/` following the CP01–CP12 format. Update this map's
"YOU ARE HERE" marker as items land.