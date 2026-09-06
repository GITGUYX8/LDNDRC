# Checkpoint 11 — GPU Requests Become Optional

## The big picture first

Bob's session pod sat in Pending with `Insufficient nvidia.com/gpu` because
every session demanded one GPU time-slice, and the k3d demo node has no GPU.
On real Host laptops that demand is correct; on a CPU-only demo cluster it
made every session unschedulable. This checkpoint made the GPU request
conditional: present by default, omitted when `SESSION_GPU_LIMIT=0`.

Think of it as a venue with and without a stage lift — the show can go on in
a smaller room, it just skips the lift act.

## What got built

| File | Friendly name | What it does |
|---|---|---|
| `control-panel/internal/sessions/kubernetes.go` | The Flexible Rider | Adds `nvidia.com/gpu` to requests/limits only when the limit is set and not `"0"`. |
| `control-panel/internal/sessions/kubernetes_test.go` | The Two-Room Exam | Asserts GPU present at `"1"` (requests and limits) and absent at `"0"` with CPU/memory intact. |
| `manifests/control-panel-deployment.yaml` | The Demo Setting | Sets `SESSION_GPU_LIMIT: "0"` with a comment marking production Hosts as `"1"`. |
| `local_dev_docs/checkpoint-11-*.md` | This Story | Records the change and the live-cluster outcome. |

## How each piece works

### The Flexible Rider — `kubernetes.go`

The resource maps are built with CPU and memory first; the GPU key is added
to both only when `p.gpuLimit` is non-empty and not `"0"`. The constructor
default stays `"1"`, so any deployment that does not set the variable keeps
the old behavior exactly. This mirrors the reference project's
`SESSION_GPU_LIMIT` rule.

### The Two-Room Exam — `kubernetes_test.go`

The existing test now also checks GPU limits at `"1"`. A new test provisions
with `"0"` through the fake client and asserts no `nvidia.com/gpu` key in
either map while CPU and memory remain.

### The Demo Setting — deployment manifest

One env value flipped to `"0"`, documented inline. Reverting is one line.

## How it all connects

1. Control-panel image rebuilt, imported, rolled out on the live cluster.
2. Bob's pre-change session (GPU request baked in) deleted; fresh session
   created through the live API.
3. Scheduler places the pod (nodeSelector already satisfied by the Host
   label); local-path binds the 5Gi PVC.
4. Kubelet attempts the workspace image pull → `ImagePullBackOff`, the
   honest next blocker (image doesn't exist yet).
5. Session stays `provisioning`; gateway keeps returning 503 — correctly
   refusing traffic to a workspace that isn't ready.

## Proof it works

Observed:

- `gofmt -l` clean; `go vet` exit 0; `go test ./...` all `ok`, including
  the new `TestKubernetesProvisionerOmitsGPUWhenLimitIsZero` (verified with
  `-run GPU -v`: PASS).
- `go build ./...` exit 0.
- Live Deployment JSON: `requests`/`limits` contain only CPU and memory —
  no `nvidia.com/gpu` key.
- Live pod moved Pending → `ImagePullBackOff`; PVC `Bound` 5Gi via
  local-path; gateway editor host with valid cookie → 503.
- Stale pre-change session resources (Deployment/Service/PVC) removed via
  kubectl since the restarted control panel's in-memory store no longer
  knew them — a known limitation, persistence is future work.
- `git diff --check` clean.

## The one-liner version

| Piece | One-liner |
|---|---|
| `kubernetes.go` | GPU requests appear only when the cluster actually wants them. |
| `kubernetes_test.go` | Proves both rooms: GPU on at `"1"`, GPU gone at `"0"`. |
| Deployment manifest | Demo cluster opts out of GPUs with one documented line. |

## What's next

- [ ] Option 2 follow-up: tiny stand-in workspace image so a session reaches
      `Running`/`ready` for full gateway 200-path verification.
- [ ] Or the real thing: build the Jazzy/Harmonic workspace image on GPU
      hardware.
- [ ] Session store persistence so restarts don't orphan K3s resources.
- [ ] Commit this checkpoint's three code files when ready.
