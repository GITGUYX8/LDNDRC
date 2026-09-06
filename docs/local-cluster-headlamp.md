# Local Cluster Runbook (k3d + Headlamp)

Run the whole LDNDRC stack on this laptop: K3s cluster (via k3d), control
panel, demo session, Traefik ingress, and the Headlamp dashboard.
Tested on Omarchy/Arch. Only the steps marked "needs sudo" require
privileges; everything else runs as your normal user.

> Current demo state: sessions use the tiny `demo-standin` image and
> `SESSION_GPU_LIMIT=0` because this machine has no GPU. Production Hosts
> use the Jazzy/Harmonic workspace image and `"1"`. See
> `local_dev_docs/checkpoint-11-gpu-optional.md` and
> `local_dev_docs/checkpoint-12-demo-standin-ready-path.md`.

## 0. Prerequisites (one time, needs sudo)

```bash
sudo usermod -aG docker hari
sudo systemctl enable --now docker
newgrp docker        # or log out and back in
docker ps            # must list containers without error
```

User-local tools (no sudo, via mise):

```bash
mise install go@latest kubectl@latest k3d@latest
mise exec k3d@latest -- k3d version
```

## 1. Create the cluster

```bash
mise exec k3d@latest -- k3d cluster create ldndrc \
  -p "80:80@loadbalancer" -p "443:443@loadbalancer"
mise exec kubectl@latest -- kubectl get nodes
```

## 2. Build and import images

```bash
docker build -t ldndrc/control-panel:dev ./control-panel
docker build -t ldndrc/demo-standin:dev ./images/demo-standin
mise exec k3d@latest -- k3d image import ldndrc/control-panel:dev ldndrc/demo-standin:dev -c ldndrc
```

## 3. Deploy the control plane

```bash
mise exec kubectl@latest -- kubectl create namespace ldndrc
mise exec kubectl@latest -- kubectl -n ldndrc create secret generic ldndrc-control-panel \
  --from-literal=JWT_SECRET="$(openssl rand -hex 32)"
mise exec kubectl@latest -- kubectl apply -f manifests/control-panel-rbac.yaml
mise exec kubectl@latest -- kubectl apply -f manifests/control-panel-deployment.yaml
mise exec kubectl@latest -- kubectl apply -f manifests/control-panel-service.yaml
mise exec kubectl@latest -- kubectl apply -f manifests/control-panel-ingress.yaml
mise exec kubectl@latest -- kubectl -n ldndrc rollout status deployment/ldndrc-control-panel
```

Verify RBAC scope (expect `yes`, `yes`, `no`):

```bash
mise exec kubectl@latest -- kubectl -n ldndrc auth can-i create deployments \
  --as=system:serviceaccount:ldndrc:ldndrc-control-panel
mise exec kubectl@latest -- kubectl -n ldndrc auth can-i delete persistentvolumeclaims \
  --as=system:serviceaccount:ldndrc:ldndrc-control-panel
mise exec kubectl@latest -- kubectl -n ldndrc auth can-i list nodes \
  --as=system:serviceaccount:ldndrc:ldndrc-control-panel
```

## 4. Deploy Headlamp

```bash
mise exec kubectl@latest -- kubectl apply -f manifests/headlamp.yaml
mise exec kubectl@latest -- kubectl -n ldndrc rollout status deployment/ldndrc-headlamp
```

Login token (paste into the Headlamp login screen):

```bash
mise exec kubectl@latest -- kubectl -n ldndrc create token ldndrc-headlamp --duration=24h
```

## 5. Expose locally

```bash
mise exec kubectl@latest -- kubectl -n ldndrc port-forward svc/ldndrc-control-panel 8082:8082 &
mise exec kubectl@latest -- kubectl -n ldndrc port-forward svc/ldndrc-headlamp 8080:80 &
curl http://127.0.0.1:8082/healthz   # expect {"status":"ok"}
curl -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8080/  # expect 200
```

Open the dashboard: **http://localhost:8080** → paste the token → cluster `main`.

Optional: real hostnames instead of `curl -H "Host: ..."` (needs sudo):

```bash
# add to /etc/hosts, with <LAN-IP> = this machine's LAN address
# <LAN-IP> control.ros-platform.local editor.ros-platform.local \
#   desktop.ros-platform.local gazebo.ros-platform.local
```

## 6. Create a session and test the gateway

```bash
TOKEN=$(curl -s -X POST http://127.0.0.1:8082/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"bob","password":"secret123"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
curl -s -X POST http://127.0.0.1:8082/api/sessions -H "Authorization: Bearer $TOKEN"
mise exec kubectl@latest -- kubectl -n ldndrc get pods   # session pod -> Running
```

Gateway checks (need the login cookie):

```bash
COOKIE=$(curl -s -D - -o /dev/null -X POST http://127.0.0.1:8082/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"bob","password":"secret123"}' \
  | grep -i "^Set-Cookie:" | head -1 | sed 's/^Set-Cookie: //; s/;.*//')
curl -s -H "Host: editor.ros-platform.local" -H "Cookie: $COOKIE" http://127.0.0.1:8082/  # 200
curl -s -o /dev/null -w "%{http_code}\n" -H "Host: editor.ros-platform.local" http://127.0.0.1:8082/  # 401
```

## 7. Troubleshooting

| Symptom | Meaning | Fix |
|---|---|---|
| Session pod `Pending`, `didn't match node affinity` | Node lacks `role=host` label | `kubectl label node k3d-ldndrc-server-0 node-role.kubernetes.io/role=host` |
| Session pod `Pending`, `Insufficient nvidia.com/gpu` | GPU requested but no GPU present | Expected unless `SESSION_GPU_LIMIT=0` is set (demo default) |
| Session pod `ImagePullBackOff` | Image not imported into k3d | Re-run the `k3d image import` step |
| Session stuck `provisioning`, gateway 503 | Pod not Ready yet | Check `kubectl -n ldndrc describe pod <name>` events |
| Port-forward dead after rollout restart | Forward was tied to old pod | Re-run the port-forward commands |
| `kubectl` tries `localhost:8080` | No cluster context | `k3d cluster list`; recreate if missing |

## 8. Teardown

```bash
mise exec k3d@latest -- k3d cluster delete ldndrc
```

This removes the whole cluster (workloads, PVC data, secrets). Re-run from
step 1 to start fresh. The JWT Secret is generated per deploy and never
committed — see `manifests/control-panel-secret.example.yaml` for its shape.
