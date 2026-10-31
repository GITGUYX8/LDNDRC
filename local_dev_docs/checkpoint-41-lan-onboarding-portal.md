# Checkpoint 41 — LAN onboarding portal + download counter

## What landed

The "LAN Connection Approval Gate": a captive-style onboarding portal inside the
control panel binary, reachable pre-approval, with a live served-download counter and
two enrollment scripts. Soft-portal flow only — gateway enforcement is a designed
follow-up (contract documented, not implemented), per the architecture decision:
master laptop hosts the onboarding AP + portal, the separate cluster router carries all
cluster traffic.

## Files

| File | The Desk | Notes |
|---|---|---|
| `internal/portal/store.go` (new) | The Counter | JSON DB (`DOWNLOADS_DB`, default `/var/lib/ldndrc/downloads.json`): requests vs downloads vs hashed-devices, mutex + atomic tmp+rename writes, persisted salt keeps device hashes stable across restarts. |
| `internal/portal/handler.go` (new) | The Turnstile | Portal page, script downloads (request-vs-served counting), public count + JWT-gated stats. `countWriter` verifies the full body actually reached the wire. |
| `internal/portal/assets/` (new) | The Handout | `portal.html` (standalone, inline CSS/JS, Mission-Control palette, 3s polling counter), `joining-cluster.sh`, `joining-cluster.ps1` — `go:embed`. |
| `internal/httpapi/router.go` | The Front Desk, extended | New `portalH *portal.Handler` param; portal routes registered before the SPA fallback; `/api/downloads/stats` JWT-gated via the existing `bearerClaims`. |
| `cmd/server/main.go` | The Wiring | Builds the portal store+handler; **degrades gracefully** — an unreadable downloads DB only disables the portal, it does not log.Fatal (unlike JWT/nodes). |
| `internal/nodes/store.go` | The Token Maker | `newID()` 4 → 16 random bytes. Node ID is now the enrollment bearer capability (128-bit), satisfying "secure enrollment tokens, not IP/MAC identity" with zero API churn. |
| `internal/httpapi/portal_test.go` (new) | | Routes: page served (not SPA), trailing-slash variant, download counted through the router, stats 401 without / 200 with token. |
| `internal/portal/{store,handler}_test.go` (new) | | Persistence across reload, concurrency exactness (8×12 parallel increments), unknown-artifact guard, unique-device union math, public-count privacy (no `requests`/`devices` fields), HEAD/Range-are-requests-only. |
| `internal/nodes/store_test.go` | | `TestFreshNodeIDIsEnrollmentTokenStrength` — 32-hex-char IDs, distinct per laptop. |
| `web/src/api.ts` + `web/src/pages/Admin.tsx` | The Instructor Console | `getDownloadStats()` (optional — older panels degrade to `undefined`); Fleet metrics gain "script downloads" + "unique downloaders" tiles. `npm run build` re-embedded. |
| `manifests/control-panel-deployment.yaml` | | `DOWNLOADS_DB=/var/lib/ldndrc/downloads.json` env, commented emptyDir caveat. |
| `docs/lan-onboarding-portal.md` (new) | | Full design: layout, sequence, counter semantics/caveats, privacy, AP setup, gateway-enforcement contract. |
| `README.md` | | Path B §2b (portal flow), API table rows, file-tree entries. |
| `test-joining.sh` (new, repo root) | | 5 threshold scenarios against `joining-cluster.sh --detect-only` (mirrors `test-join.sh`, no network). |

## Counting semantics (the part that must not be misunderstood)

- `requests` = every hit of a script endpoint (HEAD, `Range: bytes=0-` resume probes,
  304 revalidations, aborted transfers).
- `downloads` = full `200` GET with the entire body written. A retry cannot inflate it;
  a browser preview doesn't count; a *successful* download counts as both (requests
  include downloads).
- Neither number proves the user saved or executed the file. The portal page states
  this verbatim; so does `docs/lan-onboarding-portal.md`.
- Unique devices = distinct `sha256(salt‖IP‖UA)` per artifact; `unique_devices` = union
  across artifacts. IP/UA are spoofable and documented as directional; MACs are not
  used; raw values are never stored or served.

## Why the scripts live in `internal/portal/assets` (not repo root)

`go:embed` needs a compile-time-stable path, and a root copy would drift from the
served one. `join-cluster.sh` (the legacy gatekeeper) stays untouched at the root;
`joining-cluster.sh` is the new portal enrollment agent and has a different job:
register → poll approval → switch Wi-Fi → (Hosts, optional) k3s agent install with a
confirm prompt.

## Proofs

- `gofmt` clean; `go build ./...` OK; `go vet` OK; `go test -count=1 ./...` green
  (all packages; join 10s).
- `bash -n joining-cluster.sh` clean; `--detect-only` exercised with Host (12/16/nvidia),
  Guest (4/8/none), boundary RAM=15 → Guest, and the real machine.
- `test-joining.sh`: 5/5 pass.
- `npm run build`: tsc + vite clean, dist rebuilt with the two new metric tiles.
- Portal routes verified through the router test suite: page wins over the SPA
  fallback, `/portal/` variant works, count/stats behave, stats requires JWT.
- **Live end-to-end (panel binary + real script + real approval):**
  - Portal page, script download (correct `Content-Type` +
    `Content-Disposition: attachment`), HEAD + Range probes counted as requests
    only (`requests=4, downloads=2` after 2 GETs + HEAD + 0-20 probe), public
    count aggregate-only, stats 401 unauthenticated / full detail with JWT,
    `downloads.json` persisted with salt + one hashed device.
  - **Approval path**: script registered (32-char ID), instructor approved via
    `POST /api/nodes/{id}/approve`, script observed approval within one poll
    cadence, printed the Wi-Fi manual step, exit 0.
  - **Denial path** (`TEST_HOSTNAME` override → fresh fingerprint): script
    reported `DENIED by the instructor: laptop does not qualify`, exit 1.
  - **JOIN_KEY path**: panel with `JOIN_KEY` → script without the key →
    `join key rejected` immediately.
  - **Idempotency**: two consecutive script runs registered the same node id
    (server-side fingerprint dedupe), node count stayed 1.

## Bugs the E2E caught (fixed in the same checkpoint)

1. `NODE_ID="" POLL_BODY="" PANEL_URL=""` clobbered the user's `PANEL_URL`
   env var before `main()` ran — discovery fell through to the prompt every
   time. Globals line removed.
2. `log()` output inside `register`/`wait_approval` was captured by command
   substitution (`body=$(...)`), so diagnostics vanished and poll bodies got
   polluted with log lines. Both functions now hand results back via
   `REG_BODY`/`POLL_BODY` globals and log to stdout.
3. The poll used `curl -f`, so the approved-node `503 join service
   unavailable` (no minter outside the cluster / `K3S_JOIN_URL` unset) was
   swallowed as a transport failure — approval was never observed. The poll
   now parses every response; those two 503 bodies are recognized as
   "approved, token not mintable yet" and the flow proceeds with a clear
   caveat. The same 503 semantics are documented in `control-panel/README.md`
   (pre-existing behavior).
4. The "Registered as node …" line hardcoded `pending`; it now reports the
   server's actual status (matters on re-registration of approved/denied
   nodes).
5. Added `TEST_HOSTNAME` override (alongside `TEST_*` hardware overrides) so a
   second laptop's fingerprint — and therefore the deny path — is testable on
   one machine.

## Not done here (deliberate)

- Gateway/firewall enforcement: contract only (`GET /api/gateway/devices` sketch in the
  design doc). Enforcement stays at the gateway/router; approval state and enforcement
  remain decoupled so any router integration bolts on without touching the core.
- Redirect/interception of arbitrary client HTTP: soft portal by design; the AP's
  dnsmasq wildcard-DNS trick (documented) covers most browser attempts.
- Node records don't store MAC/last-IP yet — needed only when a gateway integration
  lands; the design doc notes where they'd go.
