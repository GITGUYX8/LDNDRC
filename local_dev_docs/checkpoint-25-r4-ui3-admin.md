# Checkpoint 25 — R4 UI-3: Instructor Console (detailed)

## Context

Continues on branch `feat/r3-csrf`, unpushed. Checkpoint-24 left the
live master running the UI-1/UI-2 image (verified GREEN) and queued
UI-3 next. The design doc (`docs/uiux-design.md` §8) scopes UI-3 as
"Admin table, instructor view, session metrics", §7 names the
reference pattern explicitly — **"Admin page (user/session cards)"**,
verdict "Defer … UI in a later phase". UI-3 *is* that later phase, so
this run implements the reference **card pattern**, not a plain data
table. No backend changes were needed: `GET /api/nodes`,
`POST /api/nodes/{id}/approve`, `POST /api/nodes/{id}/deny`, and
`GET /api/sessions` already exist, Bearer-authed with CSRF on
mutations. Scope per operator: build UI-3, commit locally, no push;
dirty join/nodes files left untouched.

## Reference docs considered

- `docs/uiux-design.md` §3/§7/§8 — admin = user/session cards (the
  pattern kept here); student IA unchanged (`/admin` is additive, no
  route moved); Mission Control tokens, dot+text status, real
  `<button>`s, 2px focus rings, `prefers-reduced-motion` (inherited
  from the base stylesheet).
- `ldndrc-workshop-design.md` §8 — session record fields (id, user,
  status set, node, ROS_DOMAIN_ID, created_at) all surface on the
  session cards; the `status → list` admin index is mirrored
  client-side as status filter chips.
- `docs/roadmap.md` R4 — UI-3 is the admin view closing out the
  UI-1/UI-2/UI-3 phasing.
- `local_dev_docs/checkpoint-10-headlamp-visuals.md` — gating
  headlamp behind an ADMIN role stays future work, untouched.
- `sim-web-app/README.md` cockpit notes — student-cockpit concern,
  not admin; untouched.

## What shipped (staged for this checkpoint's commit)

### `control-panel/web/src/api.ts` — nodes seam

- `NodeStatus` (`pending approved denied joined expired`) + `JoinNode`
  (field names match the Go JSON: `ram_gb`, `node_name`,
  `requested_at`, …).
- `listNodes()` / `approveNode(id)` / `denyNode(id, reason)` through
  the shared `request()` wrapper (Bearer + `X-CSRF-Token` +
  `credentials: include`), so the 403/401 semantics from checkpoint-24
  apply unchanged.

### `control-panel/web/src/pages/Admin.tsx` — `/admin` (new, auth-gated)

Reference-pattern cards throughout (reuses the Launch page's
`.cards`/`.card` language):

- Header: back-to-launch link, title, manual ↻ Refresh.
- **Fleet metrics strip**: host count + per-status counts
  (dot + text label, never color-alone), session count +
  per-status counts.
- **User cards**: one per account with avatar initial, session count,
  ready/provisioning summary. Backend note: `GET /api/sessions` is
  scoped to the caller (`Store.List(username)` has no list-all seam),
  so today this is one card for the signed-in instructor; a list-all
  endpoint would populate the whole room here with **no view change**.
- **Session cards** with status filter chips (all/provisioning/ready/
  stopping/stopped/error + counts): status dot+label + live age,
  id, user, node, ROS domain id, created time, error `<details>`
  when present, Stop button for active own sessions.
- **Host cards**: hostname, specs (os/arch/CPU/RAM/GPU), node name,
  requested time, deny reason when present; Approve (one click) /
  Deny… (inline reason input + confirm) for `pending` (approve also
  for `expired`, matching the store's transition); otherwise no
  actions.
- Auto-refresh every 5s, error banner, toasts; empty states explain
  the join-TUI flow.

### Wiring (no backend change)

- `App.tsx`: `/admin` route behind `RequireAuth`; SPA fallback
  (`spa.go`) already serves `index.html` for it — verified `/admin`
  200 locally.
- `Launch.tsx` READY panel: muted "Instructor console →" link.
- `TopBar.tsx`: "Admin" ghost button next to the user menu.
- `styles.css`: `.status--bad` (danger dot, was missing),
  `.admin/.panel--wide/.admin__head`, `.metrics/.metric`,
  `.cards--admin`, `.facts`, `.stack`, `.chips/.chip`,
  `.usercard__avatar`, `.input` — Mission Control tokens only.

## Proofs

- `tsc` + `vite build` green (47 modules, bundle
  `index-lFfXNHjr.js` 196.68KB/61.79KB gzip + CSS 8.97KB).
- `gofmt -l` clean, `go vet ./...` clean, `go test` ok (auth,
  httpapi, nodes, sessions; full suite green in checkpoint-24 run).
- Local server smoke (temp NODES/SESSIONS_DB, no cluster): `/admin`
  200 with the new bundle; login → `GET /api/nodes` shows the
  registered `card-laptop` pending record, `POST /api/sessions`
  202 — the exact payloads the cards render.
- `dist/.gitkeep` restored after the vite `emptyOutDir` wipe
  (same discipline as checkpoint-23); built assets gitignored.
- Dirty-tree discipline: the 8 join/nodes modifications +
  `join-cluster.sh` unmodified and unstaged; untracked junk
  (`ldndrc-join-linux-amd64`, `session-ses_f638.md`,
  `ldndrc-workshop-design.md`) left alone.

## Not done (next)

Rebuild image → `ctr import` (sudo) → delete-old-first rollout →
verify `/admin` on `control.ros-platform.local` (200 + bundle, user/
session/host cards with live data, approve/deny round-trip). Then
R2-final (nodes/sessions → PVC/DB) + panel Deployment
`maxUnavailable: 1` (live manifest still predates both). Optional
later: `GET /api/admin/sessions` list-all seam to fill the Users
section room-wide; ADMIN-role gating for headlamp (checkpoint-10).
