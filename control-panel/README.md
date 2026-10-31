# LDNDRC Control Panel

The control panel is the Go service that authenticates users and creates one
private ROS 2 workspace per session. When it runs inside K3s, its ServiceAccount
uses namespace-scoped RBAC to create a Deployment, Service, and home PVC in the
`ldndrc` namespace.

## Build the image

Build from this directory and make the image available to the K3s nodes:

```bash
docker build -t ldndrc/control-panel:dev .
```

Or with podman:

```bash
podman build -t ldndrc/control-panel:dev .
```

For a k3d cluster, import the image into the cluster after building it:

```bash
k3d image import ldndrc/control-panel:dev -c <cluster-name>
```

## Deploy on K3s

Create the JWT secret without committing it:

```bash
kubectl create namespace ldndrc
kubectl -n ldndrc create secret generic ldndrc-control-panel \
  --from-literal=JWT_SECRET="$(openssl rand -hex 32)"
```

Apply RBAC and the control-panel workload:

```bash
kubectl apply -f ../manifests/control-panel-rbac.yaml
kubectl apply -f ../manifests/control-panel-deployment.yaml
kubectl apply -f ../manifests/control-panel-service.yaml
```

The namespace creation is harmless if it already exists. The Secret must exist
before the Deployment can become ready.

## Verify

```bash
kubectl -n ldndrc rollout status deployment/ldndrc-control-panel
kubectl -n ldndrc get pods,svc,serviceaccount
kubectl -n ldndrc auth can-i create deployments \
  --as=system:serviceaccount:ldndrc:ldndrc-control-panel
kubectl -n ldndrc port-forward svc/ldndrc-control-panel 8082:8082
curl http://127.0.0.1:8082/healthz
```

Expected health response:

```json
{"status":"ok"}
```

## Host onboarding (nodes API)

Laptops register join requests; an operator approves them and the laptop
polls until it receives a short-lived K3s bootstrap token:

```bash
# laptop → register (unauthenticated, rate-limited, optional JOIN_KEY)
curl -X POST http://127.0.0.1:8082/api/nodes/register \
  -H 'Content-Type: application/json' \
  -d '{"hostname":"laptop-a","os":"linux","arch":"amd64","cpu":12,"ram_gb":32,"gpu":"NVIDIA RTX"}'
# → {"id":"<id>","status":"pending"}

# operator → approve (JWT from /api/auth/login)
curl -X POST http://127.0.0.1:8082/api/nodes/<id>/approve \
  -H "Authorization: Bearer $TOKEN"

# laptop → poll until approved; first poll mints the token (once)
curl http://127.0.0.1:8082/api/nodes/<id>
# → {"status":"approved","k3s_url":"...","k3s_token":"K10...","node_name":"ldndrc-<id>"}
```

Requires the nodes RBAC (`control-panel-rbac.yaml`), the `NODES_DB` volume,
and `K3S_JOIN_URL` set to the master's reachable address
(e.g. `https://192.168.1.50:6443`) — without it, approved polls mint the
token but answer 503. A 5s watcher labels Ready nodes `role=host` and marks
them `joined`.

## LAN onboarding portal

Pre-approval portal served from the same binary (design + gateway
follow-up contract: `docs/lan-onboarding-portal.md`):

```bash
# any laptop on the onboarding Wi-Fi, before approval:
open http://127.0.0.1:8082/portal
curl -O http://127.0.0.1:8082/portal/joining-cluster.sh   # Linux/macOS
curl -O http://127.0.0.1:8082/portal/joining-cluster.ps1  # Windows

# live counter (public, aggregate only):
curl http://127.0.0.1:8082/api/downloads/count
# → {"artifacts":{"joining-cluster.ps1":0,"joining-cluster.sh":2},"total":2}

# operator view incl. request counts + unique-device estimate (JWT):
curl -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8082/api/downloads/stats
```

Counts persist in `DOWNLOADS_DB` (default `/var/lib/ldndrc/downloads.json`,
same emptyDir caveats as `NODES_DB`). "Requests" (HEAD/Range/retries) are
counted separately from full served downloads, and neither proves the
student saved or ran the file — the portal page says so explicitly.

The session image must already be available to the K3s nodes. A session stays
in `provisioning` until its Deployment reports an available replica, at which
point the store promotes it to `ready` and the gateway starts routing to it.

## Reference-aligned gateway hosts

The reference project routes browser tools through the control panel using
hostnames rather than path prefixes. Apply `../manifests/control-panel-ingress.yaml`
after making these names resolve to the Traefik entry point:

```text
control.ros-platform.local
editor.ros-platform.local
desktop.ros-platform.local
gazebo.ros-platform.local
```

The current Go gateway authenticates the `editor`, `desktop`, and `gazebo`
hosts with the HttpOnly session cookie, then resolves the authenticated user's
ready session Service. The Deployment sets `COOKIE_DOMAIN=.ros-platform.local`
so the browser sends the cookie to all four subdomains. The legacy
path-based `ros2-ingress.yaml` was removed after these host routes were
verified live on K3s.
