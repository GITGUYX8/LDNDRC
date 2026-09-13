# Checkpoint 20 — Real K3s Master Cutover + Baseline (detailed)

## Context

First run on a real K3s server on this laptop (`hari`, `192.168.1.37`,
K3s `v1.36.4+k3s1`, containerd `2.3.4`), replacing the k3d `ldndrc`
demo cluster. Scope was master-only by decision: cutover, redeploy,
baseline verify, docs. The guest full-agent join is explicitly deferred
to a later paired run. No push this run (HEAD stays local).

Prior state (checkpoint-19): k3d demo, `K3S_JOIN_URL` rehearsal value,
`ADVERTISE_MDNS=false`, no `:6443` listener, TUI Join correctly locked.

## Step-by-step with observed outputs

### 1. JWT recovery (k3d briefly revived, then stopped again)

```bash
k3d cluster start ldndrc
kubectl -n ldndrc get secret ldndrc-control-panel -o jsonpath='{.data.JWT_SECRET}' | base64 -d > /tmp/ldndrc-jwt  # 64 hex chars, mode 600
k3d cluster stop ldndrc   # :80/:443 confirmed free afterwards
```

Recovery succeeded — no fresh secret needed, all existing operator
tokens stay valid. Rollback preserved: `k3d cluster start ldndrc` +
`kubectl config use-context k3d-ldndrc` restores the demo world (its
kubeconfig context was left untouched).

### 2. Lifeboats + kubeconfig + images (one sudo block, user-run)

- `docker save` both images to `/tmp/*.tar` (the fork session's
  tarballs were gone — `/tmp` is tmpfs; images re-saved from docker).
- User ran: kubeconfig export (`sudo cat /etc/rancher/k3s/k3s.yaml >
  ~/.kube/k3s-real.yaml`, server `https://127.0.0.1:6443`), both
  `k3s ctr images import` (confirmed via `crictl`: `control-panel:dev`
  `7a45b2fef6cc4`, `demo-standin:dev`), `get nodes` (`hari Ready
  control-plane`, INTERNAL-IP `192.168.1.37`).
- Firewall verified from `ufw status`: `6443/tcp`, `80,443/tcp`,
  `5353/udp`, `8472/udp` all ALLOW (v4+v6), plus a pre-existing
  `ALLOW FWD 10.42.0.0/24` pod-network forward rule.

### 3. Redeploy (order matters)

1. `control-panel-rbac.yaml` — 8 resources created (namespace, SA,
   session Role + binding, nodes Role + binding in kube-system,
   nodes ClusterRole + binding).
2. Secret from `/tmp/ldndrc-jwt`, verified round-trip
   (`SECRET-MATCHES-RECOVERED`).
3. Deployment with `ADVERTISE_MDNS=true` (manifest flip, committed).
4. **Correction:** `kubectl apply` on
   `control-panel-hostnetwork-patch.yaml` fails — the file is a merge
   fragment, not a full object (`spec.selector: Required value`).
   Applied the identical intent via `kubectl patch --type=strategic`
   `{"hostNetwork":true,"dnsPolicy":"ClusterFirstWithHostNet"}`.
   Do not `apply` that file; the patch command is the procedure.
5. Service, Ingress (traefik, 4 hosts, ADDRESS `192.168.1.37`),
   Headlamp (ghcr.io pull OK, `1/1 Running`).
6. Read-back: `hostNetwork=true`, `dnsPolicy=ClusterFirstWithHostNet`,
   `ADVERTISE_MDNS=true`, `K3S_JOIN_URL=https://192.168.1.37:6443`.
   First real evidence: panel pod IP is `192.168.1.37` (the host).

### 4. Baseline incident A — session stuck Pending (documented path)

New session stayed `provisioning`; pod `Pending`:
`didn't match Pod's node affinity/selector` — fresh master lacks the
`role=host` label. Fix per README/runbook/checkpoint-09 precedent:

```bash
kubectl label node hari node-role.kubernetes.io/role=host
```

Pod scheduled, PVC `Bound` (`local-path`, 5Gi).

### 5. Baseline incident B — editor 502 via wildcard DNS (new finding)

With the pod Running and session `ready`-capable, gateway `editor`
returned 502 `workspace service unavailable` while pod-direct and
ClusterIP curls both returned 200. Replicated with a `busybox` pod in
the panel's exact posture (`hostNetwork:true`,
`ClusterFirstWithHostNet`): `nslookup` correct (`10.43.44.89`), but
`wget` dialed **`185.38.109.203`** — the LAN's wildcard DNS answer.

Mechanism: the host's DHCP search suffix (`domain.name`) is inherited
by the pod's `resolv.conf`; the target has 4 dots < `ndots:5`, so the
stub resolver walks the search list first and the wildcard hit
short-circuits the correct CoreDNS reply. k3d never saw this (docker
DNS, no such suffix). Fix (one line + comment,
`session_gateway.go:67`): trailing dot makes the upstream a FQDN so
the search list is skipped:

```go
"http://" + session.ServiceName + "." + ns + ".svc.cluster.local.:" + port
```

### 6. Rollout incident — hostNetwork port deadlock (new finding)

`rollout restart` with the fixed image deadlocked: new pod `Pending`
(`didn't have free ports`), old pod holding `:8082`, `maxUnavailable:0`
waiting for a ready successor that could never schedule. Fixed by
deleting the old panel pod (~40s API downtime). **Side effect:**
in-memory sessions wiped (known R2 debt); the verify session's K8s
resources orphaned exactly as documented, then cleaned by hand
(deploy+svc+pvc deleted). Future rollouts on a single hostNetwork node
need `maxUnavailable:1` or the same delete-old-first step.

### 7. Baseline GREEN (new session `fc4fc8545676`, via Traefik `:80`)

| Check | Observed |
|---|---|
| Session `provisioning` → `ready` | via gateway `Current()` flip |
| `editor` + cookie | **200 + stand-in body** (first time on this master) |
| `desktop` + cookie | 502 by design |
| `editor` no cookie | 401 |
| Cookie-domain lesson | login must go through `control.ros-platform.local` (cookie `Domain=.ros-platform.local` is rejected for `127.0.0.1` logins by curl's jar — harness artifact, not a bug) |
| mDNS | **`discovery: advertising _ldndrc-master._tcp on hari:8082`** — first real advertisement in project history |

## What this proves (and what it doesn't)

Proves: full stack (RBAC, recovered Secret, hostNetwork panel with
mDNS, Traefik ingress, Headlamp, per-user sessions, gateway matrix)
runs on the real master with zero k3d in the path; two first-time
findings (wildcard-DNS trap, hostNetwork rollout deadlock) diagnosed
and fixed/documented.

Does not prove: guest agent join (deferred — needs the paired run with
`TEST_*` overrides), guest-side mDNS sighting (`avahi-browse` from the
second laptop still pending), real workspace image (still stand-in),
persistence across restarts (R2, observed live again here).

## Follow-ups

- Paired guest join: register → approve → token → toolkit-skip →
  real `k3s agent` install → watcher-joined → session schedules on the
  new node; plus first `avahi-browse -r _ldndrc-master._tcp` sighting.
- Consider `maxUnavailable:1` (or Recreate) for the panel Deployment
  so single-node rollouts never deadlock again.
- Journey report + roadmap updated alongside (same checkpoint commit).
