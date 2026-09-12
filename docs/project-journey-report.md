# LDNDRC Project Journey — Done vs Remaining (Visual Report)

> Companion to `docs/roadmap.md` (R-item tracker) and
> `local_dev_docs/detailed-project-report.md` (technical inventory). This
> document is the *visual journey* layer: where we started, every era we
> passed through, what is live today, and what remains — as coloured
> workflow diagrams with ASCII fallbacks (Mermaid renders on GitHub/GitLab).

## Colour legend (used in every diagram)

| Colour | Meaning |
|---|---|
| Green | Built **and** verified live on the cluster or real hardware |
| Amber | Built but provisional — simulated, demo-scoped, or awaiting hardware proof |
| Red | Missing or blocked — real remaining work |
| Grey | Removed or superseded along the way |

## 1. The journey flowchart (start → today → remaining)

```mermaid
flowchart LR
    classDef done fill:#1d5c2e,stroke:#3ecf8e,color:#fff
    classDef prov fill:#6b4e12,stroke:#f5b86c,color:#fff
    classDef missing fill:#5c1d1d,stroke:#f56c6c,color:#fff
    classDef gone fill:#2a2d34,stroke:#6b7280,color:#ccc

    GEN["Genesis<br/>plan.md + workspace images<br/>join-cluster.sh + tests"]:::done
    FND["Foundation CP01–CP04<br/>join logic, containers,<br/>Go panel, manifests + UI design"]:::done
    SES["Sessions CP05–CP06<br/>per-user Deployments<br/>panel on K3s"]:::done
    GW["Gateway CP07 + R1<br/>cookie + host routing<br/>legacy paths removed"]:::done
    VER["Verification CP08–CP09<br/>static + live K3s"]:::done
    OBS["Observability CP10<br/>Headlamp + LAN view"]:::done
    DEMO["Demo-ready CP11–CP12<br/>optional GPU + stand-in"]:::prov
    ONB["Onboarding CP14–CP19<br/>nodes API, join binary,<br/>releases, mDNS, rehearsal"]:::prov
    YOU["YOU ARE HERE<br/>Sep 2026, main green"]:::prov

    REAL["Real K3s master<br/>+ full agent join"]:::missing
    FEAT["R2 persistence<br/>R3 CSRF · R4 frontend"]:::missing
    IMG["R6 real image<br/>R7 hardening"]:::missing
    P56["P5 Windows · P6 docs"]:::missing

    OLD1["Fixed StatefulSet model"]:::gone
    OLD2["Path routing /code /sim"]:::gone
    OLD3["systemd master plan"]:::gone
    OLD4["v0.1.0 unwired Join"]:::gone

    GEN --> FND --> SES --> GW --> VER --> OBS --> DEMO --> ONB --> YOU --> REAL --> FEAT
    REAL --> IMG
    FEAT --> P56
    ONB -.-> P56
```

```text
ASCII fallback:

plan.md/genesis -> CP01-04 foundation -> CP05-06 sessions -> CP07+R1 gateway
  -> CP08-09 verification -> CP10 observability -> CP11-12 demo-ready
  -> CP14-19 onboarding => YOU ARE HERE
  => real-master join => R2/R3/R4 features (+ P5/P6 alongside) => R6/R7
Removed along the way: StatefulSet model, path routing, systemd plan, v0.1.0
```

**Era notes (one line each).** Genesis: design docs, ROS/Gazebo image
recipes, the bash join gatekeeper with its 4-scenario exam. Foundation:
Go control panel skeleton, manifests, UI/UX design. Sessions: per-user
Deployment/Service/PVC with unique ROS domains, panel deployed on K3s.
Gateway: cookie auth + host-subdomain routing, legacy paths deleted after
live proof. Verification: static suite green, then the first live-K3s
proofs. Observability: Headlamp in-cluster, viewed from the guest laptop
over LAN. Demo-ready: GPU made optional, 82MB stand-in image proves the
ready path. Onboarding: nodes API, join binary + TUI, three releases,
mDNS advertiser, wired Join screen, first real laptop-to-master handshake.

## 2. System architecture map (live state, Sep 2026)

```mermaid
flowchart TB
    classDef done fill:#1d5c2e,stroke:#3ecf8e,color:#fff
    classDef prov fill:#6b4e12,stroke:#f5b86c,color:#fff
    classDef missing fill:#5c1d1d,stroke:#f56c6c,color:#fff
    classDef gone fill:#2a2d34,stroke:#6b7280,color:#ccc

    Guest(["Guest laptop<br/>browser + ldndrc-join v0.2.1"]):::done
    LAN["Same Wi-Fi LAN<br/>192.168.1.x"]:::done
    Traefik["Traefik :80/:443<br/>k3d loadbalancer"]:::done
    CP["Control panel (Go)<br/>auth · sessions · nodes<br/>gateway · discovery"]:::done
    Sess["Per-user Deployment<br/>Service 7682/8080/9002<br/>home PVC 5Gi"]:::done
    WS["Workspace pod<br/>demo-standin TODAY"]:::prov
    Head["Headlamp<br/>LAN :8080 + token"]:::done
    K3S["K3s API (k3d)<br/>client-go provisioner"]:::done

    RealM["Real K3s master<br/>serves :6443"]:::missing
    RealWS["Jazzy/Harmonic image<br/>~10GB+ build"]:::missing
    UI["Student frontend<br/>login/launch/workspace"]:::missing
    Persist["Shared persistence<br/>sessions + nodes"]:::missing
    CSRF["CSRF tokens"]:::missing

    Dead["Fixed StatefulSet<br/>Path routing<br/>systemd unit"]:::gone

    Guest --> LAN --> Traefik --> CP
    CP --> K3S --> Sess --> WS
    Guest --> Head
    CP -.-> RealM
    Sess -.-> RealWS
```

```text
ASCII fallback:

Guest laptop --LAN--> Traefik :80 --> control panel --> K3s API
    --> per-user Deployment/Service/PVC --> workspace pod (stand-in TODAY)
Guest --> Headlamp :8080 (operator view, token-gated)
Missing: real master :6443, real workspace image, student frontend,
  shared persistence, CSRF. Removed: StatefulSet model, path routing, systemd.
```

Live snapshot backing this map: control-panel, headlamp, and Bob's demo
session pods all 1/1 Running; per-session Service with 7682/8080/9002 and
bound 5Gi PVC; Traefik ingress on all four hostnames; LAN forwards `:8082`
(API) and `:8080` (Headlamp); gateway verified editor-200 / desktop-502 /
no-cookie-401; nothing listens on `:6443` (k3d API is localhost-only).

## 3. Onboarding swimlane (as built and proven)

```mermaid
sequenceDiagram
    participant L as Joining laptop
    participant T as Traefik :80
    participant C as Control panel
    participant O as Operator (human)
    participant K as K3s API
    L->>T: POST /api/nodes/register (hardware fingerprint)
    T->>C: forward (any Host falls through to API)
    C-->>L: pending + node id
    O->>C: POST approve (JWT, live from here)
    L->>C: GET poll every 5s
    C->>K: create bootstrap-token Secret, TTL 15m
    C-->>L: approved + K10 token + k3s_url + node_name (once)
    L->>C: GET poll again
    C-->>L: status only (single-mint proven)
    O->>K: delete test Secret (cleanup)
    Note over L,K: Proven over real Wi-Fi (checkpoint-19).<br/>Agent install + real master join: pending real hardware.
```

```text
ASCII fallback:

laptop --register--> master (pending + id) | operator approves
  | laptop polls --> master mints one 15-min token --> token once
  | later polls --> status only | operator deletes test secret.
Proven on real Wi-Fi; agent install awaits a real K3s master.
```

Colour status of this flow: register/approve/mint/cleanup green (live);
TUI-driven variant amber (wired + unit-tested, rehearsal pending second
terminal); agent install + watcher-joined red (needs real master).

## 4. Verification ledger (claim → proof → observed output)

| Claim | How proven | Observed |
|---|---|---|
| One workspace per user, isolated | Unit + live: two users, distinct Deployments/Services/PVCs, ROS domains 100/101, owner-scoped lists | `kubectl get` shows separate resources; cross-user access 403 |
| Gateway routes correctly | Live via Traefik with Host headers | editor 200 (stand-in body), desktop/gazebo 502 by design, no-cookie 401, unready 503, old `/code` 404 |
| Scheduler enforces Host + GPU rules | Live scheduler events | affinity refusal unlabeled; `Insufficient nvidia.com/gpu` on GPU-less node; schedules when GPU optional |
| Sessions clean up fully | Live delete | Deployment + Service + PVC all gone; other user's untouched |
| Bootstrap tokens mint once | Live across real Wi-Fi | one Secret (correct type/expiry/groups/description), second poll token-free, zero left after cleanup |
| Join binary parity | `--plain` vs `test-join.sh` | byte-for-byte on all 4 scenarios; shell suite 4/4 |
| TUI quits + transitions | Unit + pty run | `q` exits 0 with bye; keypress matrix green; cross-compiles 3 OSes (~7.5MB) |
| mDNS announce→find | Unit round trip | passes; LAN proof needs real master (advertiser off in k3d by design) |
| Releases installable | Fresh download each tag | checksum OK; binary runs first try on guest |
| RBAC least privilege | Live `auth can-i` probes | session rights yes; node reads no (base); minter/watcher rights yes |

## 5. Decisions + debt register

Pivots (each with its rationale recorded in checkpoints): Go over NestJS;
per-session resources over fixed StatefulSet; cookie+hosts over paths
(R1); in-cluster Deployment over root systemd (report amended); bootstrap
Secrets over CLI-minted tokens; conditional GPU over faked capacity;
stand-in scoped to plumbing proof; Bubble Tea over heavy UI frameworks;
darwin guest-only; no silent installs and no Guest overrides (locked D1/D4);
plain Actions releases; `MASTER_IP` required, never defaulted.

Known debt (all acknowledged, none hidden): in-memory/emptyDir stores
orphan resources on restart (R2 will subsume nodes JSON too); stand-in is
not a workspace (R6); no student frontend (R4); no TLS/CSRF (R3/security);
`K3S_JOIN_URL` rehearsal value; no real master join yet; Windows/WSL2 and
LAN-mDNS unproven on hardware; legacy `join-cluster.sh` still the only
complete join path.

## 6. Remaining work funnel (ordered, with dependencies)

1. **Paired TUI rehearsal** — second terminal drives the wired Join screen
   through approval + token receipt; abort at sudo prompt (no installs).
2. **Real K3s master + full agent join** — unblocks `:6443`, watcher-joined,
   P2 LAN-mDNS proof, session scheduling on new node.
3. **R3 CSRF → R2 persistence → R4 frontend** — secure, durable, visible.
4. **P5 Windows/WSL2 + P6 deprecation/docs.**
5. **R6 real image on GPU hardware → R7 hardening** (TLS, quotas, policy,
   observability).

Guide: finish items top-down; each row's proof is a checkpoint report in
`local_dev_docs/`; move the roadmap's YOU-ARE-HERE marker as rows land.
Related docs: `docs/roadmap.md` (R-tracker), `local_dev_docs/`
(CP01–CP19 stories), `docs/local-cluster-headlamp.md` (runbook).
