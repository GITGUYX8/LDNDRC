# Checkpoint 21 — R3 CSRF + R2 Session Persistence (detailed)

## Context

Two roadmap items landed on the real master (`hari`), on branch
`feat/r3-csrf` (from `main@a404617`), each committed alone. No push.
R3 closed a reference-parity gap before the R4 SPA makes cookie-authed
mutations real; R2 ended the restart-orphan tax observed in
CP09/CP11/checkpoint-20. Guest agent join remains deferred.

## R3 — CSRF protection

### Scope correction (found in code, not assumed)

The planned premise — "cookie auth on mutating routes" — did not exist:
all mutating API routes require the `Authorization: Bearer` header
(`bearerClaims`), which browsers never attach cross-site. The session
cookie (`ldndrc_session`, `SameSite=Lax`) is consumed only by the
gateway's GET navigation. Classic CSRF was already neutered by
architecture; R3 is defense-in-depth plus the precondition for R4.

### What shipped (`473441f`)

- `IssueCSRF`/`VerifyCSRF` (`internal/auth`): `HMAC(JWT_SECRET,
  "csrf:"+jwt)`, hex, constant-time compare. Stateless, unforgeable
  without the secret, rotates with re-login. Token travels in JS memory
  as `X-CSRF-Token`, never a cookie.
- `guardCSRF` middleware (`internal/httpapi/csrf.go`): cookie present +
  mutating method + missing/invalid token → **403** before any handler.
  Bearer-only requests and safe methods pass through; gateway GET
  navigation untouched (R4 iframe compat).
- Login returns `csrf_token` alongside JWT + cookie.
- `POST /api/nodes/register` stays exempt: pre-auth by design,
  rate-limited (10/min/IP). Diagnostics/polls carry no cookie.

### Proofs

- Unit: 7 guard cases on real routes (403/403/401-fallthrough/foreign
  rejection/Bearer-202/GET-pass/register-open) + 5 token-binding cases.
  Full `go test ./...` green (7 packages), `gofmt`/`vet` clean.
- Live (rebuilt image, user `ctr import`, preempted hostNetwork rollout
  deadlock via delete-old-first): login shape OK; cookie POST no
  token → 403, bad token → 403, valid token w/o bearer → 401 (guard
  passed, handler auth), Bearer → 202; gateway editor-200 regression
  holds under the R3 image.

## R2 — Session persistence

### What shipped (`6241549`)

- `sessions.json` beside `nodes.json` (`SESSIONS_DB`, default
  `/var/lib/ldndrc/sessions.json`, emptyDir limits documented inline).
  Every mutation saves under lock; Create rolls back the map on save
  failure; corrupt file is a hard error (never silently start empty).
- Boot reconcile (`main.go`, after provisioner wiring): loaded
  non-terminal records checked via `WorkloadExists` (fail-open on API
  errors) → existing **adopted** (readiness re-probes lazily on next
  gateway visit), missing **tombstoned to `stopped`** with
  `"workload gone at restart"` (never silent-delete, never
  auto-reprovision). Counts logged.
- Nodes DB stays on emptyDir (PVC/DB path = R2-final follow-up).
- Manifest gains explicit `SESSIONS_DB` env for symmetry with NODES_DB.

### Proofs

- Unit: save/load round-trip + `Current` after reload, corrupt-file
  fails fast, adopt/tombstone + tombstone-persists-across-reload,
  terminal records ignored.
- Live restart proof (in-place container kill via `crictl stop`, same
  pod `restarts=0` → `1`, emptyDir intact): session `20c8e08984b1`
  listed with the **same id** post-restart (user-confirmed, then
  re-verified), `provisioning` → `ready` on gateway visit,
  editor-200 first try, zero orphans. This is the first restart in
  project history that lost nothing.

## Data-layer clarification (asked mid-run)

Two layers, one touched: session **records** (JSON pointers + status,
hundreds of bytes — persisted + reconciled) vs workspace **home data**
(5Gi PVCs, K8s-native — always survived; the bug was forgotten
pointers, now fixed). Login tokens were never at risk (secret in a K8s
Secret). Node approvals still emptyDir-only (deferred).

## Follow-ups

- R4: step 0 locate `uiux-design.md` (absent from `docs/`), then
  scaffold + phased UI-1..3.
- R2-final: nodes + sessions to PVC or DB when leaving demo stage.
- Panel Deployment: `maxUnavailable:1` (or Recreate) so single-node
  hostNetwork rollouts never deadlock (worked around twice now).
- Uncommitted join-file modifications in the tree (`cmd/join/main.go`,
  `internal/join/*`) are NOT part of R3/R2 — origin unknown, flagged
  to the operator, left untouched.
