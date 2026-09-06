# Checkpoint 6 — Putting the Control Panel Inside K3s

## The big picture first

The session builder could now describe and create a student workspace, but the
control panel itself still had no K3s home. This checkpoint gave it a small,
locked-down apartment inside the cluster and documented how to put it there.

The new deployment uses K3s's normal Kubernetes API. It does not need cluster
administrator access: it can manage only the session resources in the `ldndrc`
namespace.

## What got built

| File | Friendly name | What it does |
|---|---|---|
| `control-panel/Dockerfile` | The Packing Box | Builds the Go control panel into a small static runtime image. |
| `manifests/control-panel-rbac.yaml` | The Key Ring | Creates the namespace, ServiceAccount, and narrowly scoped permissions. |
| `manifests/control-panel-deployment.yaml` | The Control Room | Runs one authenticated control-panel replica inside K3s. |
| `manifests/control-panel-service.yaml` | The Reception Desk | Gives the control panel a stable internal address on port 8082. |
| `manifests/control-panel-secret.example.yaml` | The Secret Recipe | Shows the shape of the JWT Secret without committing a real secret. |
| `control-panel/README.md` | The Setup Card | Explains image building, secret creation, deployment, and checks. |
| `.gitignore` | The Vault Lock | Keeps the real local control-panel Secret out of git. |

## How each piece works

### The Packing Box — `control-panel/Dockerfile`

Think of this as packing the control panel for travel. A Go build stage compiles
the server into one static Linux binary, then a minimal distroless image carries
only that binary into K3s.

The runtime exposes port 8082 and runs as the non-root `nonroot` user. The image
expects the project image `ldndrc/control-panel:dev` by default in the K3s
Deployment.

### The Key Ring — `manifests/control-panel-rbac.yaml`

This is the control panel's carefully sized key ring. It creates the `ldndrc`
namespace and a dedicated ServiceAccount, then allows that account to create,
inspect, update, and remove only Deployments, Services, PVCs, and Pods in that
namespace.

It does not grant cluster-admin access. That matters because the control panel
creates student workspaces, but should not be able to alter the whole K3s farm.

### The Control Room — `manifests/control-panel-deployment.yaml`

This is the small office where the control panel works. It receives the JWT
secret from Kubernetes, turns on in-cluster mode, and points session creation at
the `ldndrc` namespace and the Jazzy/Harmonic workspace image.

It checks `/healthz` before claiming to be ready and keeps checking afterward.
It also runs with a read-only filesystem, no privilege escalation, dropped
capabilities, and a RuntimeDefault seccomp profile.

### The Reception Desk — `manifests/control-panel-service.yaml`

Think of this as the building directory. It gives the control panel a stable
ClusterIP address named `ldndrc-control-panel` on port 8082, so the future
gateway and frontend do not need to know which pod is currently running it.

### The Secret Recipe — `manifests/control-panel-secret.example.yaml`

This is an empty recipe card, not a real password. It documents the Secret
format for people who prefer applying YAML, while the README gives a safer
command that generates a random value locally.

### The Vault Lock — `.gitignore`

The real file `manifests/control-panel-secret.yaml` is now ignored. This makes
it possible to keep a local Secret manifest without accidentally adding the JWT
signing key to the repository.

### The Setup Card — `control-panel/README.md`

This is the runbook for the checkpoint. It covers building the image, importing
it into a k3d cluster, creating the JWT Secret, applying the manifests, checking
rollout status, and confirming the ServiceAccount can create session resources.

## How it all connects

1. Build `ldndrc/control-panel:dev` from the Go control-panel directory.
2. Import the image into the K3s or k3d nodes.
3. Create the JWT Secret locally, without committing it.
4. Apply the namespace and RBAC rules.
5. K3s starts the control-panel Deployment with its ServiceAccount.
6. The control panel talks to the K3s API and creates one Deployment, Service,
   and PVC for each student session.
7. The stable control-panel Service becomes the internal destination for the
   gateway work that comes next.

## Proof it works

- All four new YAML files passed Ruby YAML parsing.
- `git diff --check` passed.
- The existing hardware regression test still passed all 4 scenarios.
- Dockerfile validation could not run because access to the Docker daemon was
  denied.
- Go compilation and tests remain unverified because `go` and `gofmt` are not
  installed in this environment.
- A real K3s rollout and RBAC check are not verified yet because `kubectl` is
  not installed and no cluster is connected here.

## The one-liner version

| Piece | One-liner |
|---|---|
| `control-panel/Dockerfile` | Packs the Go server into a small non-root container. |
| `control-panel-rbac.yaml` | Gives the server only the K3s keys it needs. |
| `control-panel-deployment.yaml` | Runs the server inside K3s with health checks and secret wiring. |
| `control-panel-service.yaml` | Gives the server a stable internal address. |
| `control-panel/README.md` | Explains how to build, deploy, and verify the control panel. |

## What's next

- [ ] Install the Go and kubectl toolchains and run formatting, dependency, and
      compilation checks.
- [ ] Build/import the control-panel and workspace images into K3s.
- [ ] Apply the manifests on a real K3s cluster and verify the ServiceAccount
      permissions.
- [ ] Connect the gateway to each session's private Service.
- [ ] Add readiness reconciliation so sessions move from `provisioning` to
      `ready` after their workspace is actually available.
