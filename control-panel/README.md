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
