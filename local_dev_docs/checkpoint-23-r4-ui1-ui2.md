# Checkpoint 23 — R4 UI-1 + UI-2: Student SPA Scaffold (detailed)

## Context

R4 continues on branch `feat/r3-csrf`, unpushed. Checkpoint-22 left two
backend prerequisites (design doc restored, `Get()` self-heal, embedded SPA
serving with placeholder). This checkpoint builds the actual student
frontend: UI-1 (Login → Launch stepper → Workspace shell) plus UI-2 polish
(Desktop wrapper, reconnect, split persistence, a11y/motion) per
`docs/uiux-design.md` §§4–6,8. Scope per operator: UI-1+UI-2, dirty
join/nodes files left untouched, commit locally, no push.

## What shipped (uncommitted until this checkpoint's commit)

### UI-1 — `control-panel/web/` scaffold (Vite + React 18 + TS)

- `package.json` / `vite.config.ts` / `tsconfig.json` / `index.html`:
  React 18.3 + react-router-dom 6 + Vite 5, `base: "/"` (served from the
  control host root), dev proxy for `/api` + `/healthz` → `:8082`.
- `src/api.ts`: fetch wrapper — Bearer token on all session calls,
  `X-CSRF-Token` on mutations, `credentials: "include"` for the session
  cookie; `login()` returns `{token, csrf_token}`.
- `src/auth.tsx`: memory-first token/csrf/user state with sessionStorage
  mirror (reload-safe), cross-tab sign-out.
- `src/gateway.ts`: host-gateway URL helper — `control.ros-platform.local`
  → `editor|gazebo|desktop.ros-platform.local` (path routing is legacy).
  Unknown suffix (localhost/IP dev) yields `""` and an honest banner.
- `src/App.tsx`: `/` → `/launch` or `/login`; `/login /launch /app
  /desktop` with auth gate; SPA fallback already covered by `spa.go`.
- `pages/Login.tsx`: Mission Control card, inline validation, show/hide,
  error banner, no reload, no sign-up (instructor-provisioned).
- `pages/Launch.tsx`: NO_SESSION hero → PROVISIONING stepper (polls
  `GET /api/sessions/{id}` every 2s, terminates via the CP22 self-heal;
  Cancel = DELETE) → READY tool cards + Open workspace → ERROR panel with
  `<details>` + Retry; elapsed timer.
- `pages/Workspace.tsx` + `TopBar` / `Rail` / `SplitStage` pattern:
  48px top bar (brand + Code/Sim/Split segmented + dot+label+timer +
  Desktop-new-tab + Stop-confirm + user menu), 56px icon rail (real
  `<button>`s), stage with Code iframe (`editor`) / Sim iframe (`gazebo`)
  / Split with draggable divider. Both iframes stay **mounted** across
  view switches (CSS visibility, hidden-pane trick) so code-server and sim
  sockets never reconnect on view change.
- `styles.css`: Mission Control tokens verbatim (`--bg #0b0e14`, `--accent
  #7c6cf5`, …), Inter + JetBrains Mono, 150–200ms ease-out, 2px accent
  focus rings, status dot + text label.

### UI-2 — polish in the same run

- `pages/Desktop.tsx`: Selkies wrapper full-bleed + Back-to-workspace
  pill, keyboard-capture hint once (localStorage `ldndrc.desktopHintDismissed`).
- Reconnect: rail ↻ button + badge, `Reconnecting…` toast, iframe `key`
  bump; dimmed-empty states with retry when the suffix is unknown.
- Split ratio persisted (`ldndrc.splitRatio`, clamped 0.2–0.8, keyboard
  arrows on the divider); `prefers-reduced-motion` disables pulse and
  transitions; below-1024px view-switch buttons (desktop-first per design
  open question 4).
- Sim is an **iframe** to the gazebo gateway host in UI-1/UI-2 (fast,
  matches the CP22 "iframes" note). The mounted gzweb component from
  `sim-web-app/` is deferred to R6 alongside the real workspace image.

### Build integration

- `npm run build` → `web/dist/` (`index.html` + 1 JS 188KB/59.9KB gzip +
  1 CSS 7.3KB); `tsc` clean.
- `Dockerfile`: new `node:22-bookworm-slim` webbuild stage (`npm install`
  + `npm run build`, placeholder fallback) + `COPY --from=webbuild`
  overlay, so images always ship the real UI even though `web/dist/`
  stays gitignored. `COPY . .` already covered `web/` — no other change.
- `dist/.gitkeep` stays tracked (restored after the vite emptyOutDir
  wipe); built assets remain ignored by design — source is committed,
  output is reproducible.

## Proofs

- `tsc` + `vite build` green (46 modules).
- `gofmt -l` clean, `go vet ./...` clean, `go test -count=1 ./...` all
  `ok` (auth, discovery, gateway, httpapi, join, nodes, sessions).
- Local server smoke (in-memory provisioner, no cluster): `/`, `/login`,
  `/app?view=code` all serve the built SPA; `/healthz` ok;
  `/api/sessions` 401 without bearer; login → token (153 chars) →
  `POST /api/sessions` 202 provisioning → list/get round-trip.
- Dirty-tree discipline: the 8 join/nodes modifications + `join-cluster.sh`
  flagged in checkpoint-21 are **unmodified and unstaged**; untracked junk
  (`ldndrc-join-linux-amd64`, `session-ses_f638.md`,
  `ldndrc-workshop-design.md`) left alone.

## API surface the SPA uses (unchanged, all live)

`POST /api/auth/login` → `{token, csrf_token}` + cookie ·
`POST /api/sessions` (202) · `GET /api/sessions` · `GET /api/sessions/{id}`
(self-healing) · `DELETE /api/sessions/{id}` · gateway hosts
(editor-7682/gazebo-9002/desktop-8080, cookie-authed, 401/404/503/502
mapping) · `GET /` SPA fallback for `/login /launch /app /desktop`.

## Next (live verify on the real master, not done in this run)

Rebuild image → `ctr import` → **delete-old-first** rollout (hostNetwork
`:8082` deadlock, never plain `rollout restart`) → via
`control.ros-platform.local`: login → launch → stepper to ready via
gateway visit → editor iframe 200 + stand-in body, desktop 502-by-design,
no-cookie 401, cookie POST without `X-CSRF-Token` 403. Then UI-3 (admin
table) per the design doc; R2-final (nodes → PVC/DB); panel Deployment
`maxUnavailable: 1`.
