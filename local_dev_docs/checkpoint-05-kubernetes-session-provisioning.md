# Checkpoint 5 — The Per-Student Kubernetes Workspace

## The big picture first

The earlier design had three shared workspace pods. That was not enough for the
real promise of LDNDRC: each student needs their own private ROS 2 room, their
own files, and their own browser connections. This checkpoint connected the
session idea to K3s by teaching the control panel how one session should become
one Kubernetes workspace.

The implementation follows the useful pattern from the reference project: one
session gets one Deployment, one Service, and one home-directory storage claim.
The Kubernetes part is kept behind a small doorway so the rest of the control
panel can still be tested without a live cluster.

## What got built

| File | Friendly name | What it does |
|---|---|---|
| `control-panel/go.mod` | The Tool List | Adds the official Kubernetes client libraries needed to talk to K3s. |
| `control-panel/internal/sessions/store.go` | The Session Register | Tracks who owns each workspace, its status, and its private ROS channel. |
| `control-panel/internal/sessions/kubernetes.go` | The Workspace Builder | Creates and removes the Deployment, Service, and PVC for a session. |
| `control-panel/cmd/server/main.go` | The Switchboard | Uses the Kubernetes builder when the control panel is running inside K3s, while retaining local test mode. |
| `control-panel/internal/sessions/store_test.go` | The Session Register Exam | Checks ownership, duplicate sessions, lifecycle rules, and provisioning failures. |
| `control-panel/internal/sessions/kubernetes_test.go` | The K3s Blueprint Exam | Uses a fake Kubernetes client to inspect the resources that would be created. |

## How each piece works

### The Tool List — `control-panel/go.mod`

Think of this as adding the official vocabulary for speaking to Kubernetes.
The project now includes `client-go` and its API types, which work with K3s
because K3s exposes the normal Kubernetes API rather than a private one.

### The Session Register — `control-panel/internal/sessions/store.go`

This is the receptionist who keeps the seating chart. It records the student,
the generated workload and Service names, the ROS domain number, and whether
the workspace is provisioning, ready, stopping, stopped, or broken.

It allows only one active session per user, refuses cross-user lookups, and
allocates ROS domain IDs from 100 through 232 without reusing an active one.
The store can call a `Provisioner`, but the default local version stays in
memory so unit tests do not need a K3s cluster.

### The Workspace Builder — `control-panel/internal/sessions/kubernetes.go`

Think of this as a small construction crew that prepares one apartment for one
student. For every session it creates a PersistentVolumeClaim (the student’s
locker), a single-replica Deployment (the workspace itself), and a private
ClusterIP Service (the address for that workspace).

The generated workspace keeps the project’s isolation rules: it receives its
own `ROS_DOMAIN_ID`, sets `ROS_AUTOMATIC_DISCOVERY_RANGE=LOCALHOST`, and is
placed only on a Host-labelled laptop. It also carries the reusable reference
defaults for a non-root user, dropped capabilities, readiness checking, CPU and
memory limits, and a one-GPU request for NVIDIA time-slicing.

Deleting a session removes its Deployment, Service, and home PVC. If resource
creation fails part way through, resources already created in that attempt are
cleaned up and the session is marked as an error.

### The Switchboard — `control-panel/cmd/server/main.go`

This is the decision point between workshop mode and K3s mode. When the
control panel sees the in-cluster Kubernetes setting, it builds a Kubernetes
provisioner and uses it for sessions. Without that setting, it uses the
in-memory store, which keeps local development possible while the cluster is
not available.

The default image is the project’s Jazzy/Harmonic workspace image, and the
namespace, image, home-storage size, and GPU count can be configured with
environment variables.

### The Session Register Exam — `control-panel/internal/sessions/store_test.go`

These tests give the register fake students and fake construction crews. They
check that Alice cannot read Bob’s workspace, that Alice cannot create two
active workspaces, that valid status changes work, and that a failed builder
leaves a useful `error` session instead of pretending everything is fine.

### The K3s Blueprint Exam — `control-panel/internal/sessions/kubernetes_test.go`

This test uses a fake Kubernetes API instead of a real cluster. It asks the
builder to prepare a sample session, then inspects the resulting objects to
confirm that the Deployment, Service, and PVC exist and that the important
settings are present: the private ROS channel, localhost-only discovery, and
the NVIDIA GPU request.

## How it all connects

1. A logged-in user asks the control panel for a workspace.
2. The Session Register creates a unique session ID and ROS domain number.
3. The Workspace Builder creates one Deployment, one Service, and one home PVC
   in K3s.
4. The Deployment runs the existing single workspace container with
   code-server, Selkies, and the Gazebo WebSocket bridge under supervisord.
5. The Service gives the future gateway a stable, session-specific address.
6. The PVC keeps the student’s home directory separate from other sessions.
7. The session remains marked `provisioning` until the upcoming status and
   gateway work confirms that the workspace is ready.

## Proof it works

- `git diff --check` passed with no whitespace errors.
- The existing hardware test still passed all 4 scenarios.
- The new tests were written around the Go fake Kubernetes client and cover
  Deployment, Service, PVC, ROS isolation, and GPU settings.
- The Go tests were **not run** because `go` and `gofmt` are not installed in
  this environment.
- A real K3s deployment was **not run yet**. The control panel still needs
  Kubernetes RBAC, namespace setup, and gateway routing before an end-to-end
  cluster rehearsal is meaningful.

## The one-liner version

| Piece | One-liner |
|---|---|
| `store.go` | Keeps every student’s session, ownership, status, and ROS channel straight. |
| `kubernetes.go` | Turns one session into one isolated K3s workspace with storage and GPU access. |
| `main.go` | Chooses real K3s provisioning in-cluster and lightweight local mode elsewhere. |
| `kubernetes_test.go` | Checks the K3s workspace blueprint without needing a cluster. |
| `store_test.go` | Proves students cannot collide with or enter one another’s sessions. |

## What’s next

- [ ] Add Kubernetes RBAC and a control-panel Deployment/Service for K3s.
- [ ] Connect the gateway to each session’s Service and proxy HTTP/WebSocket
      traffic only for the owning user.
- [ ] Add readiness/status reconciliation so `provisioning` becomes `ready`
      after the workspace actually starts.
- [ ] Run the full flow on K3s with the reusable Jazzy/Harmonic workspace image.
- [ ] Replace the remaining shared StatefulSet routing path after per-session
      routing is verified.
