# Checkpoint 03 — The Go Control Panel

## The big picture first

Until now the platform had no "brain" — just containers, manifests, and a
routing sign. This checkpoint decided what that brain should be made of and
built its foundation. We picked **Go** for the control-plane backend (the
service that will manage student sessions, log people in, and shuttle browser
traffic into the right workspace) instead of the heavy Node-based alternative
we found in the reference material. We also taught git which files are secrets
it must never commit.

## What got built

| File | Friendly name | What it does |
|---|---|---|
| `control-panel/` (Go module, 280 lines) | The Brain | The control-plane API server for sessions, login, and routing |
| `reusable-components-report.md` §5 | The Decision Log | Records why the backend is Go, not Node |
| `plan.md` Phase 5 | The Blueprint | Official plan entry for the Go control panel |
| `.gitignore` | The Vault Door | Keeps secrets and build junk out of git |

## How each piece works

### The Brain — `control-panel/`

A small Go server made of four rooms:

- **The Front Desk** (`cmd/server`) — starts the server on port 8082 and
  refuses to run without a secret key.
- **The ID Office** (`internal/auth`) — issues and checks login passes (JWT
  tokens, the digital passes browsers carry).
- **The Route Board** (`internal/httpapi`) — the wall of endpoints: health
  check, login, and a slot where session management plugs in.
- **The Bridge** (`internal/gateway`) — the traffic tunnel that will one day
  carry browser traffic into a workspace pod.
- **The Session Desk** (`internal/sessions`) — the future manager of workspace
  pods; right now it's a friendly placeholder that says "no cluster connected."

The clever part: the heavy cluster-talking machinery is kept behind thin
doorways (called seams), so the server builds and runs today — with or without
a cluster. We proved this in the smoke test.

### The Decision Log — `reusable-components-report.md`

The old plan said "skip the control panel, too heavy for MVP." This checkpoint
changed that: keep the control panel, but build it in Go. A Go binary is tiny
(about 20MB, no runtime to install) and can talk to the cluster with the exact
same power as the Node version — just leaner. The frontend stays as it is:
browser tools like code-server and the 3D viewer are JavaScript/WebGL by
nature and Go cannot replace them.

### The Blueprint — `plan.md` Phase 5

The official plan now has a Phase 5: the Go control panel — module layout,
dependencies, and a table of the endpoints it will expose (create/list/delete
sessions, login, health, websocket tunnel). The plan also swapped Monaco for
code-server and fixed the Gazebo websocket settings to match the working ones
we adopted.

### The Vault Door — `.gitignore`

46 rules that keep secrets (`.env`, `.pem`, keys, kubeconfigs) and build junk
(compiled binaries, node_modules, tarballs) out of git. Verified that no
already-tracked file is accidentally hidden.

## How it all connects

```
Browser ──► login ──► The Brain issues a pass (JWT)
                    │
                    ▼
        The Route Board checks the pass
                    │
                    ▼
        The Session Desk creates a workspace pod   (future)
                    │
                    ▼
        The Bridge tunnels browser traffic to the pod  (future)
                    │
All guarded by The Vault Door so secrets never reach git
```

## Proof it works

- **It compiles clean:** `go build`, `go vet`, and `gofmt` all pass with zero
  warnings.
- **Smoke test passed:** with the server running, the health check answered
  `{"status":"ok"}`, a login with missing details was politely rejected, and a
  valid login produced a real signed pass.
- **Cluster-less by design proven:** asking for a session while no cluster is
  connected correctly returns "not found" instead of crashing — the seams work.
- **The Vault Door verified:** `git check-ignore` confirms `.env`, the compiled
  Go binary, and node_modules are all hidden, and no existing tracked file is
  caught by mistake.
- Not yet verified: talking to a real cluster (creating pods) — the seam is
  ready but the cluster-side piece is the next step.

## The one-liner version

| Piece | One-liner |
|---|---|
| `control-panel/` | The Go brain for sessions, login, and browser routing |
| `reusable-components-report.md` §5 | Records "backend = Go, frontend stays JS" |
| `plan.md` Phase 5 | Official blueprint for the Go control panel |
| `.gitignore` | The vault door keeping secrets out of git |

## What's next

- [ ] Finish the original Task 4: copy the gzweb 3D viewer into `sim-web-app/`
- [ ] Write the real farm manifest (StatefulSet + Service + Ingress)
- [ ] Wire the Go session desk to a real cluster (client-go pod provisioning)
- [ ] Build the images and rehearse the full browser path on the test cluster