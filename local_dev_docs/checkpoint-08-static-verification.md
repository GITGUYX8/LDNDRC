# Checkpoint 8 — Static Verification and Manifest Cross-Check

## The big picture first

The gateway code was written but never compiled or tested here — the Go
toolchain was missing and no cluster was connected. Before asking a real K3s
cluster to run any of it, we needed proof that the code at least builds,
passes its unit tests, and that the YAML manifests agree with each other.
This checkpoint did all the verification possible without root privileges.

Think of it as the safety inspection before the first test drive: engine
starts, brakes work, and the road signs point at real destinations. The test
drive itself still needs keys we do not have yet (root access for Docker/K3s).

## What got built

| File | Friendly name | What it does |
|---|---|---|
| `control-panel/go.mod` + `go.sum` | The Finished Shopping List | Pins every direct and indirect Go dependency so builds are reproducible. |
| `control-panel/internal/httpapi/router.go` | The Straightened Desk | Formatting fix only — the login handler indentation is now `gofmt`-clean. |
| `control-panel/internal/sessions/kubernetes.go` | The Aligned Blueprint | Formatting fix only — struct field alignment is now `gofmt`-clean. |
| `control-panel/README.md` | The Corrected Map | Build docs now show Docker and podman, list all four hostnames, and describe the real provisioning-to-ready flow. |
| `local_dev_docs/checkpoint-08-*.md` | This Story | Records what was verified, how, and what is still blocked. |

No behavior changed in this checkpoint. The two Go files were reformatted
only; every line does exactly what it did before.

## How each piece works

### The Finished Shopping List — `control-panel/go.mod` + `go.sum`

Running `go mod tidy` (via a user-local Go 1.27.1 installed with mise, no
sudo needed) filled in the missing indirect dependencies for `client-go` and
friends, plus their checksums. Before this, `go vet`, `go test`, and
`go build` refused to run at all — the equivalent of trying to cook with half
the ingredients list torn off.

### The Straightened Desk — `router.go` formatting

The login handler had drifted one indent level to the right. `gofmt` spotted
it, and the fix is whitespace-only. This matters because unformatted code
fails CI gates and hides real diffs inside cosmetic noise.

### The Aligned Blueprint — `kubernetes.go` formatting

Same story: struct field alignment (the GVR variables, labels, probe fields)
did not match `gofmt`. Whitespace-only fix, verified clean afterward with
`gofmt -l` reporting nothing.

### The Corrected Map — `control-panel/README.md`

Three small staleness fixes: the build section assumed podman (this machine
has a Docker binary instead, so both are shown); the hostname list omitted
`desktop.ros-platform.local`; and a paragraph still claimed sessions stay in
`provisioning` until a "next chunk" that already landed in checkpoint 7.

## How it all connects

1. User-local Go toolchain installed via mise (no root required).
2. `gofmt -w` cleaned the two flagged files; `gofmt -l` confirms clean.
3. `go mod tidy` completed the dependency list.
4. `go vet ./...` passes with exit 0.
5. `go test ./...` passes: gateway, httpapi, and sessions packages all `ok`.
6. `go build ./...` passes with exit 0.
7. Every manifest parses as YAML (11 files, 15 documents).
8. Cross-references checked with a script: namespace, ServiceAccount,
   Role/Binding, Service selector, ingress backends, cookie domain, and
   session image all agree.
9. Legacy `test-join.sh` passes 4/4 (run under `timeout 60`).

## Proof it works

Actual observed results:

- `gofmt -l ./control-panel/` → clean, no output.
- `go vet ./...` → exit 0.
- `go test ./...` → `ok` for `gateway`, `httpapi`, `sessions`; no-test
  packages (`cmd/server`, `auth`) reported as such.
- `go build ./...` → exit 0.
- YAML parse of all 11 manifest files → 15 documents, all with expected
  kind/name pairs.
- Structural cross-check → ServiceAccount `ldndrc-control-panel` in
  namespace `ldndrc`; Service selector matches Deployment pod labels;
  all four ingress hosts route to Service `ldndrc-control-panel` port
  `http`; Deployment env carries `COOKIE_DOMAIN=.ros-platform.local`,
  `GATEWAY_HOST_SUFFIX=ros-platform.local`, session image
  `ldndrc/ros2-gz:jazzy-harmonic-workspace`.
- `timeout 60 bash test-join.sh` → 4 passed, 0 failed, exit 0.
- `kubectl apply --dry-run=client` → **not possible**: kubectl v1.37.0 was
  installed user-local via mise, but even client-side dry-run attempts API
  discovery against `localhost:8080` and fails with no cluster.
- Image builds and live K3s rollout → **blocked**: Docker daemon socket is
  `root:docker`, user `hari` is not in the `docker` group, the daemon is
  `inactive`, and `sudo` needs a password, so no container build or
  k3d/K3s cluster could be started from here.

## The one-liner version

| Piece | One-liner |
|---|---|
| `go.mod` / `go.sum` | The dependency list is complete, so vet, tests, and builds run. |
| `router.go` / `kubernetes.go` | Whitespace-only formatting fixes; behavior unchanged. |
| `README.md` | Docs now match the code: both runtimes, four hostnames, real ready flow. |
| Manifest cross-check | Every name, label, selector, and backend reference agrees. |
| Live cluster | Still blocked on root privileges for Docker/K3s. |

## What's next

- [ ] Privileged step (needs your action): add `hari` to the `docker` group
      or start the Docker daemon, then build `ldndrc/control-panel:dev`.
- [ ] Privileged step: bring up K3s (or k3d) on this machine and export a
      kubeconfig this user can read.
- [ ] Apply the deployment bundle: RBAC → Secret → Deployment → Service →
      Ingress, then `rollout status` and `/healthz` via port-forward.
- [ ] Configure hostname resolution for the four `.ros-platform.local` names
      (hosts file, dnsmasq, or `sslip.io`).
- [ ] Run the two-user gateway test: separate Deployment/Service/PVC per
      session, both reach `ready`, cross-user access blocked, stopped
      session returns `503`.
- [ ] After green verification, remove the legacy `ros2-ingress.yaml` path
      routes and write the live-cluster results into checkpoint 9.
