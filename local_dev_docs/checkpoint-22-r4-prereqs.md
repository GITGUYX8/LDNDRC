# Checkpoint 22 — R4 Steps 1+2: Design Doc + Backend Prerequisites (detailed)

## Context

R4 (student frontend) started with its planned step 0/1/2 on branch
`feat/r3-csrf`, unpushed. No UI code yet — this checkpoint records the
spec recovery and the two backend prerequisites UI-1 needs. R3 + R2 are
live on the real master (checkpoint-21); guest join still deferred.

## Step 1 — `uiux-design.md` restored

The 285-line design draft (Sep 1, Mission Control theme, IA, state
machine, screens, tokens, React+Vite+TS + `embed.FS` plan, UI-1..3
phasing) existed at `cdac976`, vanished from the tree, now lives at
`docs/uiux-design.md` verbatim plus a provenance header. Three deltas
vs current architecture recorded at the top (not silently rewritten):

1. Path routing (`/code /sim`, nginx) → host gateway; the SPA is served
   by the Go binary itself on the control host `/`, no sidecar.
2. `GET /api/sessions/{id}` never refreshed readiness — fixed below.
3. Login now returns `csrf_token` (R3) — SPA keeps it in memory, sends
   `X-CSRF-Token` on cookie-authed mutations.

## Step 2 — backend micro-fixes (unit-tested, uncommitted pending UI-1)

### 2a. `Get()` heals provisioning → ready (`sessions/store.go`)

The launch stepper polls `GET /api/sessions/{id}`, which used `Get()` —
no readiness probe, so the stepper would spin forever. `Get` now runs
the same lazy `StatusProvider.Ready` check as `Current()` when the
record is provisioning. Locking mirrors `Current` (RLock → copy →
RUnlock → probe → `setStatus`). Tests: flips when the fake reports
ready, stays provisioning when not.

### 2b. SPA serving (`httpapi/spa.go`, route, `web/`)

- `GET /` catch-all → embedded `web/dist`: exact files when present,
  `index.html` fallback for client routes (`/login /launch /app`).
  `/api/*` and `/healthz` win by mux specificity (tested).
- `web/embed.go` + placeholder `dist/index.html` until the Vite build
  replaces it. `COPY . .` already covers `web/` — no Dockerfile change.
- Minimal content-type map (html/js/css/json/svg); CSRF guard passes
  safe GETs through untouched.

Verified: `gofmt`/`vet` clean, `go test -count=1` sessions + httpapi
`ok`, full suite green. Not yet live (ships with the UI-1 image).

## API surface the SPA will use (all live, all verified earlier)

`POST /api/auth/login` → `{token, csrf_token}` + cookie ·
`POST /api/sessions` (202) · `GET /api/sessions/{id}` (now
self-healing) · `DELETE /api/sessions/{id}` · gateway hosts
(editor-200/desktop-502/no-cookie-401 on the real master).

## Next

UI-1 scaffold (`control-panel/web/`, Vite+React+TS: Login → Launch
stepper → shell with Code/Sim/Split iframes, top bar, rail,
tokens.css), `npm run build` into `dist/`, Go rebuild, live verify:
login → launch → ready → iframes through the gateway. Then UI-2/UI-3
per the doc. Toolchain confirmed: node 26 + npm via mise,
`sim-web-app/` present for the later gzweb component.
