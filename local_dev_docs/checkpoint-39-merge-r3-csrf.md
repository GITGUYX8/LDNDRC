# Checkpoint 39 — Merge r3-csrf into main (detailed)

## Context

`main` had two finished lines: the join fix (`4eeffea`) + R6 image
track (`e5fe6cb`, CP-27…CP-38). `feat/r3-csrf` held R2/R3/R4
(CP-21…CP-26), local-only. Merged r3-csrf → main (single merge,
`d9d9a28`) so `main` is the one line; remaining work (login fix,
R2-final, viewer) continues on `main`. No push; branch kept until
the merged line proves itself in a deploy.

## Merge (observed)

- Clean auto-merge, no conflicts. The one watched hunk resolved
  itself: `manifests/control-panel-deployment.yaml` now carries
  both the R6 `SESSION_IMAGE` flip (`jazzy-harmonic-10f1f10c`) and
  the R2 `SESSIONS_DB` env, adjacent and intact.
- Stale-comment note (not fixed here): the manifest's Selkies
  comment still says "deferred / 502-by-design" (CP-32 era) though
  Selkies is live since CP-35. Refresh with the login-fix change.
- `dist/.gitkeep` restored after the verify build's `emptyOutDir`
  wipe (standing discipline).

## Proofs on the merge result

- `gofmt -l` clean, `go vet ./...` clean.
- `go test -count=1 ./...`: all 7 packages ok (auth, discovery,
  gateway, httpapi, join 10.2s, nodes, sessions).
- `npm run build`: 47 modules, cards bundle `index-lFfXNHjr.js`
  intact (UI-3 survived the merge byte-identical).

## Known arrival

The CSRF login lockout (stale-cookie POST to `/api/auth/login`
→ 403) lands on `main` with this merge — it lives in the merged
R3 code. Fix is next, directly on `main`: exempt the login route
in `guardCSRF` + test + rebuild + rollout + live proof.

## Dirty-tree discipline

Only this report added (staged for commit). The 5 untracked files
left alone.
