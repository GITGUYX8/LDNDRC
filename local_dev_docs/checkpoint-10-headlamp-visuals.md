# Checkpoint 10 — Headlamp Visuals for the Cluster

## The big picture first

`kubectl get` works, but it shows one resource type at a time as text. When
you want to *see* the whole farm — which pods run where, why one is stuck,
how Services connect to Deployments — a visual dashboard is far faster. This
checkpoint deployed Headlamp (the Kubernetes SIG-UI dashboard,
`ghcr.io/headlamp-k8s/headlamp`) inside the cluster, following the reference
project's proven template, renamed to LDNDRC conventions.

Think of it as the control tower window: the control panel flies the planes,
and Headlamp lets you watch them.

## What got built

| File | Friendly name | What it does |
|---|---|---|
| `manifests/headlamp.yaml` | The Control Tower | ServiceAccount, read-only ClusterRole/Binding, Deployment, and Service for Headlamp in the `ldndrc` namespace. |
| `local_dev_docs/checkpoint-10-*.md` | This Story | Records the deployment and how to open the dashboard. |

## How each piece works

### The Control Tower — `manifests/headlamp.yaml`

Five resources, adapted from the reference `headlamp-template.yaml`:

- A ServiceAccount the dashboard runs as, with its token auto-mounted.
- A ClusterRole allowing `get`/`list`/`watch` on all resources plus the
  self-review verbs Headlamp's RBAC-aware UI needs. Read-only by design.
- A ClusterRoleBinding joining the two (cluster-wide read is acceptable on
  this local dev cluster; narrow it before any shared deployment).
- A single-replica Deployment running Headlamp `-in-cluster` on port 4466 as
  non-root user 100, with probes, dropped capabilities, and modest limits.
- A ClusterIP Service on port 80. Deliberately **no ingress** — the dashboard
  is reached only via `kubectl port-forward`, so nothing leaks to the LAN.

Login uses a ServiceAccount bearer token minted with `kubectl create token`,
pasted once into Headlamp's login screen.

## How it all connects

1. `kubectl apply -f manifests/headlamp.yaml` creates all five resources.
2. `kubectl -n ldndrc port-forward svc/ldndrc-headlamp 8080:80` exposes it
   on the laptop.
3. Open `http://localhost:8080`, paste the token, pick the `main` cluster.
4. The dashboard shows the control-panel pod Running, Bob's session pod
   Pending (with the GPU scheduling event visible), and all Services, PVCs,
   and the ingress in one place.

## Proof it works

Observed live:

- `rollout status deployment/ldndrc-headlamp` → successfully rolled out,
  pod 1/1 Running.
- `curl http://127.0.0.1:8080/` → HTTP 200 (UI served).
- Bearer token against the K8s API (`/api/v1/namespaces/ldndrc/pods`) →
  HTTP 200, confirming the token Headlamp will use can actually read.
- `git diff --check` → clean.

## The one-liner version

| Piece | One-liner |
|---|---|
| `headlamp.yaml` | Puts a read-only visual dashboard inside the cluster. |
| Port-forward + token | Opens it securely on your laptop with one paste. |

## What's next

- [ ] Open the dashboard and confirm the Pending session pod's scheduling
      event reads as expected.
- [ ] Pin the Headlamp image to a digest instead of `latest`.
- [ ] Later: gate a `headlamp.<host>` route behind an ADMIN role in the Go
      gateway, matching the reference project's end state.
- [ ] Commit the checkpoint-08/09/10 working-tree changes when ready.
