# Checkpoint 24 — R4 UI-1/UI-2 Live Verify on Real Master (detailed)

## Context

Continues checkpoint-23 on branch `feat/r3-csrf` (still unpushed).
Checkpoint-23 built the student SPA + webbuild stage but left the live
verify undone. This run does exactly that: rebuild → `ctr import` →
delete-old-first rollout → full gateway matrix via
`control.ros-platform.local`. No code changes this run (image rebuild
only); the 8 dirty join/nodes files + `join-cluster.sh` remain
unmodified and unstaged, untracked junk left alone.

## Step-by-step with observed outputs

### 1. Pre-flight (all green)

- `npm run build`: 46 modules, `index-Bdyr3u-x.js` 188KB/59.9KB gzip.
- `gofmt -l` clean, `go vet ./...` clean, `go test -count=1 ./...`
  all `ok` (auth, discovery, gateway, httpapi, join 10.2s, nodes,
  sessions). Note: `go`/`kubectl` via direct
  `~/.local/share/mise/installs/*` paths — the mise shims report
  "No version is set" in this shell.
- Local docker smoke (`-p 18082`): `/` + `/login` 200 with the real
  `<script src="/assets/index-Bdyr3u-x.js">` bundle (not the
  placeholder), `/healthz` 200.

### 2. Rebuild + import + rollout

- `docker build -t ldndrc/control-panel:dev -f control-panel/Dockerfile
  control-panel/` → config `48724709…`, index `a5287bf4…` (68.9MB).
- `docker save` → `/tmp/ldndrc-control-panel-dev.tar` (14M).
- Operator ran `sudo k3s ctr images import` (sudo needs a password —
  cannot be done from this session): index `a5287bf4…` confirmed.
- Delete-old-first (never plain `rollout restart` — hostNetwork `:8082`
  deadlock, checkpoint-20 §6):
  `kubectl -n ldndrc delete pod -l app.kubernetes.io/name=ldndrc-control-panel`
  → `rollout status` success in seconds.
- New pod `…-qhshf` `1/1 Running`, `hostNetwork=true`,
  `imageID=sha256:48724709…` (new), logs:
  `sessions reconcile: 0 adopted, 0 tombstoned` +
  `discovery: advertising _ldndrc-master._tcp on hari:8082`.

### 3. Live verify GREEN (new session `2e15dafa61c3`, via Traefik `:80`)

DNS note: this machine has **no** `/etc/hosts` entries for
`*.ros-platform.local`, so plain resolution hits the LAN wildcard
(`185.38.109.200+` parking page, `301`). All checks below pin with
`curl --resolve …:192.168.1.37`. Recommend adding the four hosts
entries (needs sudo) for real browser use.

| Check | Observed |
|---|---|
| SPA `/ /login /launch /app?view=code` | 200, real bundle (`index-Bdyr3u-x.js` + CSS) |
| `/healthz` | 200 `{"status":"ok"}` |
| `POST /api/auth/login` (student1) | token 157 chars + csrf 64 hex + `ldndrc_session` cookie |
| `GET /api/sessions` no bearer | 401 |
| `POST /api/sessions` + CSRF | 202 `provisioning` → `ready` on first 10s poll |
| Cookie POST without `X-CSRF-Token` | 403 |
| `editor` + cookie | **200 + stand-in body** |
| `desktop` + cookie | 502 by design |
| `editor` no cookie | 401 |

## Known debt re-observed

- Pod delete wiped in-memory/emptyDir session records, so the three
  pre-existing session workloads (`fc4fc8545676`, `fb363ba9e344`,
  `20c8e08984b1`) are now orphaned (still Running, panel reports
  `0 adopted`). Same R2 incident as checkpoint-20 §6 — hand-cleaned
  then, left running now. R2-final (nodes/sessions → PVC/DB) resolves
  it for good.
- Live Deployment still lacks `SESSIONS_DB` env (revision predates R2)
  and still uses default RollingUpdate instead of `maxUnavailable: 1`
  — both queued below.

## Next

1. Add `/etc/hosts` entries for the four platform hosts (sudo, one line).
2. UI-3 (admin table) per `docs/uiux-design.md`.
3. R2-final (nodes → PVC/DB, sessions durable) + panel Deployment
   `maxUnavailable: 1`.
4. Hand-delete the three orphaned pre-verify session workloads when
   convenient.
