# LDNDRC — Commands, Logs & Debugging Cheatsheet

> Everything in one place: daily status, logs, API probing, frontend/backend
> builds, image rollout, session/node debugging, mDNS, Headlamp, secrets,
> cleanup, and "if X then run Y" recipes. Verified against the live master
> (`hari`, `192.168.1.37`, K3s `v1.36.4+k3s1`) on 2026-09-29.
>
> Conventions: `K=~/.kube/k3s-real.yaml` (real master), namespace `ldndrc`,
> panel `:8082`, Traefik `:80`. `sudo` needs your password (no NOPASSWD).

## 0. Shell setup (run first — saves 90% of confusion)

The `mise` shims for `go`/`kubectl` fail in non-interactive shells
("No version is set"). Use the direct binaries:

```bash
export PATH="$HOME/.local/share/mise/installs/kubectl/1.37.0:$HOME/.local/share/mise/installs/go/1.27.1/bin:/usr/bin:/bin:$PATH"
export KUBECONFIG=~/.kube/k3s-real.yaml   # REAL master (not k3d demo)
kubectl config current-context            # expect: k3s-real or similar, NOT k3d-ldndrc
kubectl get nodes -o wide                 # expect: hari Ready, INTERNAL-IP 192.168.1.37
```

Handy aliases (add to `~/.bashrc`):

```bash
alias k='kubectl'
alias kr='KUBECONFIG=~/.kube/k3s-real.yaml kubectl'
alias krn='KUBECONFIG=~/.kube/k3s-real.yaml kubectl -n ldndrc'
```

Kubeconfigs on this machine:

| File | Purpose |
|---|---|
| `~/.kube/k3s-real.yaml` | **Real K3s master** (`https://127.0.0.1:6443`). Use for everything live. |
| `~/.kube/config` | Default; currently points at stale `k3d-ldndrc` (connection refused unless k3d is running). |

```bash
kubectl config get-contexts                          # list all contexts
kubectl --kubeconfig=~/.kube/k3s-real.yaml get nodes # one-shot without export
```

## 1. Daily status (is the platform up?)

```bash
krn get pods -o wide
# Expect: ldndrc-control-panel + ldndrc-headlamp Running,
# plus one ros2-session-<id> pod per live session.

krn get svc,ingress,deploy
krn get pvc                                        # one *-home 5Gi Bound per session
krn get deploy ldndrc-control-panel -o jsonpath='{.spec.template.spec.hostNetwork}{"\n"}'
# expect: true (panel runs on host network — see §9 gotchas)

krn rollout status deploy/ldndrc-control-panel     # should say "successfully rolled out"
curl -s http://192.168.1.37:8082/healthz           # direct to panel (hostNetwork port)
# {"status":"ok"}
curl -s -o /dev/null -w "%{http_code}\n" http://control.ros-platform.local/healthz
# 200 (needs /etc/hosts entries — §9)
```

Ingress hosts (Traefik, ADDRESS `192.168.1.37`, port 80):

| Host | Serves |
|---|---|
| `control.ros-platform.local` | SPA + `/api/*` + `/healthz` |
| `editor.ros-platform.local` | code-server `:7682` of your session (cookie-authed gateway) |
| `gazebo.ros-platform.local` | sim `:9002` (cookie-authed gateway) |
| `desktop.ros-platform.local` | Selkies `:8080` (502 by design on stand-in image) |

```bash
krn get ingress ldndrc-control-panel -o jsonpath='{range .items[*].spec.rules[*]}{.host}{"\n"}{end}'
```

## 2. Logs (the core debugging tool)

### 2.1 Control panel (most important)

```bash
krn logs -l app.kubernetes.io/name=ldndrc-control-panel --tail=50
krn logs -l app.kubernetes.io/name=ldndrc-control-panel -f          # follow live
krn logs -l app.kubernetes.io/name=ldndrc-control-panel --since=10m # last 10 min
krn logs -l app.kubernetes.io/name=ldndrc-control-panel -p          # PREVIOUS container (crash debugging)
krn logs deploy/ldndrc-control-panel --tail=100                      # same via deploy name
```

What to look for in panel logs:

| Line | Meaning |
|---|---|
| `control-panel listening on :8082` | fresh (re)start — everything after is this boot |
| `sessions reconcile: N adopted, M tombstoned` | boot reconcile vs K8s Deployments (R2). `0 adopted` after a pod delete = session records wiped, workloads orphaned |
| `discovery: advertising _ldndrc-master._tcp on hari:8082` | mDNS live — joining laptops can discover |
| `discovery: advertise unavailable…` | mDNS failed (expected in `docker run`, fine on hostNetwork) |
| `GET /healthz`, `POST /api/auth/login`, … | per-request log (`METHOD path`) — trace a failing call end-to-end |
| `provision failed for <user>: …` | session Create blew up — read the error |
| `token mint failed for …` | node approve → K3s token mint failed |
| `node register failed / diagnostics save failed` | nodes API trouble |

### 2.2 Session workloads (stand-in or real workspace)

```bash
SID=2e15dafa61c3   # your session id (from /launch or API)
krn logs deploy/ros2-session-$SID --tail=80        # aggregative over pod replicas
krn logs deploy/ros2-session-$SID -c <container>   # if multi-container later
krn logs -l app=ldndrc-session --tail=20           # all sessions at once (label selector)
krn get pods -l app=ldndrc-session -o wide         # which node each session sits on
```

Pod not starting? Describe beats logs:

```bash
krn describe pod -l app.kubernetes.io/name=ldndrc-control-panel  # panel itself
krn describe deploy/ros2-session-$SID
krn get events -n ldndrc --sort-by=.lastTimestamp | tail -n 30    # cluster truth, newest last
krn get events -n ldndrc --field-selector reason=FailedScheduling # scheduling failures only
```

Classic: `0/1 nodes available: didn't match Pod's node affinity/selector`
→ fresh node missing the label (fix §10.1).

### 2.3 Headlamp + Traefik

```bash
krn logs deploy/ldndrc-headlamp --tail=30
kubectl -n kube-system logs -l app.kubernetes.io/name=traefik --tail=50  # ingress routing issues
```

### 2.4 Host-level (container runtime on the master)

```bash
sudo k3s ctr images list | grep -E 'control|standin'  # what k3s actually runs
sudo k3s crictl pods --namespace ldndrc               # runtime view (vs kubectl view)
sudo k3s crictl ps -a | head -n 20
sudo journalctl -u k3s --since '30 min ago' | tail -n 50  # K3s service itself
```

## 3. Panel API via curl (auth → sessions → nodes)

Login works with **any non-empty** username/password (no user table yet).
Cookie domain is `.ros-platform.local`, so go through the hostname —
`127.0.0.1` logins get their cookie rejected by the jar (harness
artifact, not a bug). Without `/etc/hosts`, pin with `--resolve`.

```bash
R="--resolve control.ros-platform.local:80:192.168.1.37 --resolve editor.ros-platform.local:80:192.168.1.37 --resolve gazebo.ros-platform.local:80:192.168.1.37 --resolve desktop.ros-platform.local:80:192.168.1.37"
rm -f /tmp/cj.txt   # fresh cookie jar

# --- login: token (157 chars) + csrf (64 hex) + cookie ---
curl -s $R -c /tmp/cj.txt -X POST http://control.ros-platform.local/api/auth/login \
  -H 'Content-Type: application/json' -d '{"username":"student1","password":"test123"}'
TOKEN=$(curl -s $R -c /tmp/cj.txt -X POST http://control.ros-platform.local/api/auth/login \
  -H 'Content-Type: application/json' -d '{"username":"student1","password":"test123"}' \
  | python3 -c "import sys,json;print(json.load(sys.stdin)['token'])")
CSRF=$(curl -s $R -X POST http://control.ros-platform.local/api/auth/login \
  -H 'Content-Type: application/json' -d '{"username":"student1","password":"test123"}' \
  | python3 -c "import sys,json;print(json.load(sys.stdin)['csrf_token'])")

# --- sessions CRUD ---
curl -s $R http://control.ros-platform.local/api/sessions -o /dev/null -w "no-auth:%{http_code}\n"  # 401
curl -s $R -b /tmp/cj.txt http://control.ros-platform.local/api/sessions -H "Authorization: Bearer $TOKEN" | jq .
curl -s $R -b /tmp/cj.txt -X POST http://control.ros-platform.local/api/sessions \
  -H "Authorization: Bearer $TOKEN" -H "X-CSRF-Token: $CSRF" \
  -H 'Content-Type: application/json' -d '{}' | tee /tmp/sess.json   # 202 provisioning
SID=$(python3 -c "import json;print(json.load(open('/tmp/sess.json'))['session']['id'])")
# poll to ready (self-healing GET):
for i in 1 2 3 4 5 6; do sleep 10
curl -s $R -b /tmp/cj.txt http://control.ros-platform.local/api/sessions/$SID -H "Authorization: Bearer $TOKEN" | jq -r .session.status; done
curl -s $R -b /tmp/cj.txt -X DELETE http://control.ros-platform.local/api/sessions/$SID \
  -H "Authorization: Bearer $TOKEN" -H "X-CSRF-Token: $CSRF" | jq .   # stop

# --- CSRF guard proof: cookie but no X-CSRF-Token → 403 ---
curl -s $R -b /tmp/cj.txt -X POST http://control.ros-platform.local/api/sessions \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{}' -o /dev/null -w "no-csrf:%{http_code}\n"

# --- nodes (instructor) API ---
curl -s $R -b /tmp/cj.txt http://control.ros-platform.local/api/nodes -H "Authorization: Bearer $TOKEN" | jq .
curl -s $R -b /tmp/cj.txt -X POST http://control.ros-platform.local/api/nodes/<id>/approve \
  -H "Authorization: Bearer $TOKEN" -H "X-CSRF-Token: $CSRF" -H 'Content-Type: application/json' -d '{}'
curl -s $R -b /tmp/cj.txt -X POST http://control.ros-platform.local/api/nodes/<id>/deny \
  -H "Authorization: Bearer $TOKEN" -H "X-CSRF-Token: $CSRF" \
  -H 'Content-Type: application/json' -d '{"reason":"unknown hardware"}'
# public (unauthenticated) join endpoints:
curl -s $R -X POST http://control.ros-platform.local/api/nodes/register -H 'Content-Type: application/json' \
  -d '{"hostname":"laptop-a","os":"linux","arch":"amd64","cpu":8,"ram_gb":16,"gpu":""}'
curl -s $R http://control.ros-platform.local/api/nodes/<id>   # poll: pending → approved(+k3s_url/k3s_token)
```

Expected matrix (memorize it — anything else is a bug):

| Call | Expect |
|---|---|
| login (any user/pass) | 200, `token` + `csrf_token` + cookie |
| `GET /api/sessions` no bearer | 401 |
| `POST /api/sessions` + CSRF | 202 → `provisioning` → `ready` |
| cookie POST **without** `X-CSRF-Token` | 403 |
| `GET /api/nodes` no bearer | 401 |

## 4. Gateway checks (the money path)

Gateway = cookie-authed reverse proxy to your session Service.
`?session=<id>` selects the session; first visit flips `Current()`.

```bash
curl -s $R -b /tmp/cj.txt -o /dev/null -w "editor+cookie:%{http_code}\n" "http://editor.ros-platform.local/?session=$SID"
# 200 + stand-in body ("ldndrc-demo-standin… Traefik -> cookie auth -> session Service routing works")
curl -s $R -b /tmp/cj.txt "http://editor.ros-platform.local/?session=$SID" | head -c 300
curl -s $R -b /tmp/cj.txt -o /dev/null -w "desktop+cookie:%{http_code}\n" "http://desktop.ros-platform.local/?session=$SID"
# 502 BY DESIGN (stand-in has no desktop stream)
curl -s $R -o /dev/null -w "editor-no-cookie:%{http_code}\n" "http://editor.ros-platform.local/?session=$SID"
# 401
```

| Gateway result | Meaning |
|---|---|
| editor 200 + stand-in body | full path Traefik → cookie auth → Service works |
| desktop 502 | by design on `demo-standin` (no Selkies); real image serves here |
| 401 | no/invalid cookie — log in via `control.ros-platform.local` first |
| 404 | unknown session id |
| 503 | session Service not ready yet — wait for `ready` |
| 502 on editor | upstream dial failed — check session pod Running + the wildcard-DNS trap (§9) |

## 5. Frontend (control-panel/web — Vite + React 18 + TS)

```bash
npm run build     # tsc + vite → web/dist/ (gitignored by design; source is committed, output reproducible)
npm run dev       # local dev server, proxies /api + /healthz → :8082
```

Routes: `/` → `/launch`|`/login`, `/login`, `/launch`, `/app?view=code|sim|split`, `/desktop`, `/admin` — all served by the Go binary (`embed.FS`, `spa.go` fallback to `index.html`; no nginx).

```bash
curl -s http://control.ros-platform.local/ | grep -o 'src="/assets/[^"]*"'  # which bundle is LIVE?
# index-Bdyr3u-x.js = UI-1/UI-2 · index-CjnCO27K.js = UI-3 tables · index-lFfXNHjr.js = UI-3 cards
```

Browser state keys (DevTools → Application): `ldndrc.token`, `ldndrc.csrf`,
`ldndrc.user` (sessionStorage), `ldndrc.splitRatio`, `ldndrc.desktopHintDismissed`
(localStorage). "Weird auth state" → clear sessionStorage + cookie, re-login.

Vite wipes `web/dist/.gitkeep` on every build (`emptyOutDir`) — restore it, it is tracked:

```bash
git checkout -- control-panel/web/dist/.gitkeep
```

## 6. Backend (control-panel — Go)

```bash
export PATH="$HOME/.local/share/mise/installs/go/1.27.1/bin:$PATH"
gofmt -l .            # must print nothing
go vet ./...          # must print nothing
go test -count=1 ./...  # auth, discovery, gateway, httpapi, join (~10s), nodes, sessions — all ok
go test -count=1 -run TestName ./internal/nodes/   # single package/test
```

Run the panel locally (no cluster — in-memory provisioner + temp DBs):

```bash
JWT_SECRET=testsecret1234567890testsecret1234567890abcd NODES_DB=/tmp/dbg-nodes.json \
SESSIONS_DB=/tmp/dbg-sessions.json SESSION_IMAGE=ldndrc/demo-standin:dev \
CONTROL_PANEL_PORT=18082 go run ./cmd/server
# then: curl http://127.0.0.1:18082/healthz ; /admin ; login+register flows from §3 against :18082
```

Panel env vars (live values in `manifests/control-panel-deployment.yaml`):

| Var | Live | Notes |
|---|---|---|
| `JWT_SECRET` | secret `ldndrc-control-panel` | all operator tokens die if rotated |
| `COOKIE_DOMAIN` | `.ros-platform.local` | login MUST go via hostname, not IP |
| `GATEWAY_HOST_SUFFIX` | `ros-platform.local` | gateway host derivation |
| `SESSION_IMAGE` | `ldndrc/demo-standin:dev` | prod: `ldndrc/ros2-gz:jazzy-harmonic-workspace` |
| `SESSION_HOME_STORAGE` | `5Gi` | per-session PVC (`local-path`) |
| `SESSION_GPU_LIMIT` | `0` | prod hosts: `1` |
| `NODES_DB` / `SESSIONS_DB` | `/var/lib/ldndrc/*.json` | `emptyDir` — survives container restarts, NOT pod deletes |
| `K3S_JOIN_URL` | `https://192.168.1.37:6443` | handed to approved laptops |
| `ADVERTISE_MDNS` | `true` | hostNetwork required for multicast escape |
| `CONTROL_PANEL_PORT` | `8082` | hostNetwork port — the deadlock source (§9) |

## 7. Image build → import → rollout (the release procedure)

```bash
# 1. build (from repo root; webbuild stage embeds the SPA so dist/ stays gitignored)
docker build -t ldndrc/control-panel:dev -f control-panel/Dockerfile control-panel/
docker save ldndrc/control-panel:dev -o /tmp/ldndrc-control-panel-dev.tar   # /tmp is tmpfs — re-save if lost

# 2. import into k3s containerd (NEEDS SUDO — cannot be done from restricted sessions)
sudo k3s ctr images import /tmp/ldndrc-control-panel-dev.tar
# expect: unpacking … done, index sha256:<NEW>

# 3. DELETE-OLD-FIRST rollout — NEVER plain `rollout restart` (hostNetwork :8082 deadlock, §9)
krn delete pod -l app.kubernetes.io/name=ldndrc-control-panel
krn rollout status deploy/ldndrc-control-panel --timeout=120s

# 4. confirm the NEW image actually runs (compare against the import sha)
krn get pod -l app.kubernetes.io/name=ldndrc-control-panel \
  -o jsonpath='imageID={.items[0].status.containerStatuses[0].imageID} hostNetwork={.items[0].spec.hostNetwork}{"\n"}'
krn logs -l app.kubernetes.io/name=ldndrc-control-panel --tail=5
# expect: reconcile line + `advertising _ldndrc-master._tcp on hari:8082`
```

Smoke the new image BEFORE importing (catches placeholder-UI regressions):

```bash
docker run --rm -e JWT_SECRET=testsecret1234567890testsecret1234567890abcd \
  -e SESSION_IMAGE=ldndrc/demo-standin:dev -p 18082:8082 ldndrc/control-panel:dev &
sleep 3
curl -s http://127.0.0.1:18082/ | grep -o 'src="/assets/[^"]*"'  # real bundle, not placeholder
curl -s -o /dev/null -w "/login:%{http_code} /healthz:%{http_code}\n" \
  http://127.0.0.1:18082/login http://127.0.0.1:18082/healthz
kill %1
```

## 8. Gotchas that have bitten us (read before debugging anything)

1. **HostNetwork `:8082` deadlock.** Panel runs `hostNetwork:true` (for mDNS).
   One node + `maxUnavailable:0` → `rollout restart` deadlocks (new pod Pending:
   no free ports). Fix is ALWAYS delete-old-first (§7.3). Permanent fix queued:
   `maxUnavailable: 1` on the Deployment.
2. **Pod delete wipes sessions.** `NODES_DB`/`SESSIONS_DB` live on `emptyDir`.
   Every rollout orphans the K8s workloads (`0 adopted` in logs) — they keep
   Running but the panel forgot them. Hand-delete orphans; R2-final (PVC/DB)
   fixes it for good.
3. **LAN wildcard DNS trap.** Local DHCP suffix makes short names resolve to
   `185.38.109.20x` parking IPs. Symptoms: ingress returns `301` parking page,
   or in-cluster dials hit the wildcard instead of CoreDNS. Defenses: FQDN with
   trailing dot in-cluster (already in `session_gateway.go`), `/etc/hosts`
   entries + `curl --resolve` on clients.
4. **Cookie domain.** `Domain=.ros-platform.local` is rejected for `127.0.0.1`
   logins by cookie jars. Always test through `control.ros-platform.local`.
5. **Live Deployment drift.** The live manifest predates `SESSIONS_DB` env and
   `maxUnavailable:1` — `kubectl get deploy -o yaml` is truth, the file is intent.
6. **`/tmp` is tmpfs.** Saved tarballs vanish on reboot — re-`docker save`.
7. **k3d context is stale.** Default kubeconfig points at dead `k3d-ldndrc`.
   Always `KUBECONFIG=~/.kube/k3s-real.yaml`.
8. **Backgrounded `mise exec` hangs.** Use direct kubectl binary + `setsid nohup … &`
   for port-forwards (see §11).

`/etc/hosts` fix (sudo, one-time — required for real browser use):

```bash
echo "192.168.1.37 control.ros-platform.local editor.ros-platform.local gazebo.ros-platform.local desktop.ros-platform.local" | sudo tee -a /etc/hosts
getent hosts control.ros-platform.local   # must print 192.168.1.37, NOT 185.38.109.x
```

## 9. Session debugging recipes

### 9.1 New session stuck `provisioning` (pod Pending)

```bash
krn get pods -l app=ldndrc-session -o wide
krn describe pod <session-pod> | grep -A5 -i "event\|affinity\|unschedulable"
# "didn't match Pod's node affinity/selector" → fresh master lacks the label:
kubectl label node hari node-role.kubernetes.io/role=host   # (KUBECONFIG set)
krn get pvc                                                   # *-home must reach Bound (local-path)
```

### 9.2 Session `ready` but editor 502

```bash
krn get deploy/svc ros2-session-$SID          # both exist? endpoints?
krn get endpoints ros2-session-$SID
krn logs deploy/ros2-session-$SID --tail=20   # stand-in prints the plumbing line?
curl -s http://<cluster-ip>:7682/ | head -c 200   # bypass gateway: ClusterIP from krn get svc
# ClusterIP 200 + gateway 502 → gateway/dial problem (check §9.3 wildcard trap / cookie)
```

### 9.3 Orphaned workloads (post-rollout, `0 adopted`)

```bash
krn get deploy,svc,pvc -l app=ldndrc-session   # or list all and eyeball ros2-session-*
# delete a dead session's trio (deploy+svc+pvc) by hand:
krn delete deploy,svc,pvc -l <session-label-selector>
# careful: PVC delete drops the student's home dir (5Gi local-path)
```

## 10. Nodes / join debugging (second laptop flow)

Backend state:

```bash
krn exec deploy/ldndrc-control-panel -- cat /var/lib/ldndrc/nodes.json 2>&1 | head -c 2000
# (readOnlyRootFilesystem is true but /var/lib/ldndrc is the writable emptyDir mount)
```

Join TUI + scripts:

```bash
./join-cluster.sh                 # operator entrypoint (dirty-worktree file — leave unstaged)
go run ./cmd/join                 # run TUI from source
./test-join.sh                    # automated join test
./acceptance-second-laptop.sh     # full paired-run acceptance
```

mDNS (master must log `advertising _ldndrc-master._tcp`):

```bash
avahi-browse -r _ldndrc-master._tcp     # from the SECOND laptop — first real sighting still pending
```

Join lifecycle: `register` (pending) → instructor **Approve** (`/admin` Hosts
or API §3) → laptop polls `GET /api/nodes/{id}` → `approved` + `k3s_url` +
short-lived (15 min) `k3s_token` → real `k3s agent` install → watcher promotes
to `joined`. Token IDs only are persisted, never tokens.

Diagnostics bundles (join failures, ≤1MB):

```bash
curl -s $R -X POST http://control.ros-platform.local/api/nodes/<id>/diagnostics --data-binary @bundle.tar.gz
```

## 11. Headlamp (visual cluster dashboard)

```bash
KUBECTL=~/.local/share/mise/installs/kubectl/1.37.0/kubectl
KUBECONFIG=~/.kube/k3s-real.yaml $KUBECTL -n ldndrc create token ldndrc-headlamp --duration=24h
# ^ paste into Headlamp login
setsid nohup $KUBECTL -n ldndrc port-forward svc/ldndrc-headlamp 8080:80 >/tmp/pf-head.log 2>&1 < /dev/null &
# open http://localhost:8080 → cluster `main`
setsid nohup $KUBECTL -n ldndrc port-forward svc/ldndrc-control-panel 8082:8082 >/tmp/pf-cp.log 2>&1 < /dev/null &
# ^ panel API at http://127.0.0.1:8082 (re-run after every rollout; forwards die with the old pod)
```

## 12. Secrets (JWT — all operator tokens depend on it)

```bash
krn get secret ldndrc-control-panel -o jsonpath='{.data.JWT_SECRET}' | base64 -d | wc -c  # 64 hex chars
# recovery via the old k3d demo (only if k3d still exists):
k3d cluster start ldndrc
kubectl -n ldndrc get secret ldndrc-control-panel -o jsonpath='{.data.JWT_SECRET}' | base64 -d > /tmp/ldndrc-jwt  # chmod 600!
k3d cluster stop ldndrc
```

## 13. Host machine checks

```bash
ip -4 addr show | grep 192.168.1.37   # LAN IP stable?
ufw status | grep -E '6443|80|443|5353|8472'  # 6443/tcp, 80,443/tcp, 5353/udp, 8472/udp ALLOW
df -h / /var/lib/rancher              # image pulls (10GB workspace) need room
ss -tlnp | grep -E '8082|:80 |:6443'  # who holds the hostNetwork ports?
```

## 14. Git discipline (don't lose work)

```bash
git status --short --branch           # branch must be feat/r3-csrf (unpushed)
git log --oneline -5
# DIRTY-BY-DESIGN (never stage/commit these): 8 join/nodes files + join-cluster.sh
#   control-panel/cmd/join/main.go, internal/join/*, internal/nodes/nodes_test.go,
#   internal/nodes/tokens.go, join-cluster.sh
# UNTRACKED JUNK (leave alone): ldndrc-join-linux-amd64, session-ses_f638.md, ldndrc-workshop-design.md
git add <only-your-files> && git commit -m "…"   # never `git add -A`; never push unless asked
```

## 15. "If X, run Y" quick table

| Symptom | Run |
|---|---|
| `kubectl` connection refused | `export KUBECONFIG=~/.kube/k3s-real.yaml` |
| `mise: No version is set` | §0 PATH export, direct binaries |
| Ingress returns parking page / `301` | `getent hosts control.ros-platform.local` → fix `/etc/hosts` (§8) or use `--resolve` |
| Login "works" but gateway 401 | log in via hostname (not IP); check cookie jar has `ldndrc_session` |
| Cookie POST → 403 | add `X-CSRF-Token` header (use fresh `csrf_token` from login) |
| Session stuck provisioning | §9.1 (node label → PVC → events) |
| Editor 502, ClusterIP 200 | gateway/dial issue; check pod endpoints + cookie + DNS trap |
| Desktop 502 | by design on stand-in — not a bug |
| Rollout hangs (new pod Pending) | you used `rollout restart` — delete old pod (§7.3) |
| Panel logs `0 adopted` | emptyDir wiped — orphaned workloads (§9.3), R2-final pending |
| Join TUI locked / no `:6443` | master not advertising — panel logs must show mDNS line |
| Port-forward dead | re-run §11 (dies on every rollout) |
| `sudo: a password is required` | type password in a real terminal (`sudo -v`), then re-run |
| Tarball missing from /tmp | tmpfs wiped it — re-run `docker save` (§7.1) |

## 16. Repo map (where things live)

```
control-panel/cmd/server      panel entrypoint
control-panel/cmd/join        join TUI binary
control-panel/internal/auth       JWT + CSRF + cookie
control-panel/internal/sessions   session store, K8s provisioner, gateway dial, boot reconcile
control-panel/internal/httpapi    router (login/sessions/nodes), guardCSRF, spa.go fallback
control-panel/internal/gateway    cookie-authed reverse proxy
control-panel/internal/nodes      join-request store, token minter, watcher
control-panel/internal/discovery  mDNS advertisement
control-panel/web/src         SPA (api.ts, auth.tsx, gateway.ts, pages/, components/, styles.css)
control-panel/Dockerfile      node webbuild stage + go build + distroless runtime
manifests/                    rbac, deployment, service, ingress, secret.example, headlamp, hostnetwork-patch (FRAGMENT — patch via kubectl, never apply)
images/{base,demo-standin,workspace}  container images
sim-web-app/                  gzweb test harness + cockpit integration notes (README)
docs/                         uiux-design.md (R4 spec), roadmap.md, host-joiner-tui-spec.md, local-cluster-headlamp.md
local_dev_docs/               checkpoints 01–25 + this cheatsheet
```
