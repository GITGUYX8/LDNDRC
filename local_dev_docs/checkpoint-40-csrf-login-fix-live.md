# Checkpoint 40 — CSRF login-lockout fix live (detailed)

## The bug

Re-login from a browser holding a stale `ldndrc_session` cookie →
`403 "csrf token missing or invalid"`, login handler never reached.
`guardCSRF` required a CSRF header on *every* cookie-carrying POST,
but login is the endpoint that *issues* tokens — it can never require
one. Chicken-and-egg lockout.

## The fix (on main, uncommitted until this checkpoint)

- `internal/httpapi/csrf.go`: early-return the guard for
  `POST /api/auth/login` (before the method switch), with comment
  explaining why. Header comment updated (was: "pre-auth endpoints
  carry no cookie" — wrong; login carries stale cookies).
- `internal/httpapi/csrf_test.go`: regression case in `TestCSRFGuard`
  — login POST + stale cookie + no header → 200 (the helper
  `loginTokens` was always cookieless, which is why the suite never
  caught the bug).
- `manifests/control-panel-deployment.yaml`: stale comment refresh —
  Selkies 2.0.0 is live (CP-34/35), not "deferred / 502-by-design".

## Proofs before deploy

- `gofmt`/`go vet` clean; full `go test -count=1 ./...` green (all 7
  packages; join 10.3s).
- Docker image `ldndrc/control-panel:dev` rebuilt + smoke-tested in a
  throwaway container: stale-cookie login → 200; cookie-POST without
  header → 403.

## Deploy (observed)

- First import attempt failed: tarball had been saved to `/tmp`
  (tmpfs, wiped) — re-saved to `~/ldndrc-panel-csrffix.tar` (14M).
  Lesson repeated: import tarballs live in `$HOME`, never `/tmp`.
- Scratch-pod import check: a probe pod with the image returned
  `StartError: exec "sh" not found` — that error itself proves the
  image is present in containerd (missing image would be
  `ImagePullBackOff`). The panel image is distroless: no shell.
- Rollout: delete-old-first (`kubectl delete pod -l
  app.kubernetes.io/name=ldndrc-control-panel`), new pod `vm7c9`
  Running/ready in ~13s, 0 restarts. (Plain `rollout restart` stays
  forbidden — hostNetwork `:8082` deadlock, CP-20 §6.)

## Live proof matrix (through :8082, Host=control.ros-platform.local)

| # | Scenario | Observed |
|---|---|---|
| 1 | Stale-cookie login POST | **200**, token+csrf issued |
| 2 | Garbage cookie POST /api/sessions, no header | **403** |
| 3 | No cookie, Bearer + CSRF header | **202** (session created) |
| 4 | Valid cookie + matching header | **409** — guard passed; handler's one-session-per-user conflict |
| 5 | Valid cookie + wrong header | **403** — pairing enforced |

Notes on test #3/#4 mechanics (cost me a round-trip each):
- Login sets the cookie itself (`router.go:71 SetSessionCookie`), but
  with `COOKIE_DOMAIN=.ros-platform.local` curl on 127.0.0.1 won't
  store it → looked like "no cookie". Use `--resolve
  control.ros-platform.local:8082:127.0.0.1` to test the real path.
- The wrong-header "negative" without a cookie isn't a guard test at
  all: no cookie → guard skipped → handler (409 conflict).
- CSRF pairing model: `VerifyCSRF(cookie.Value, header)` — the header
  token is verified against the *cookie's* JWT.

## Environment incident discovered mid-proof

The laptop's WiFi IP drifted `192.168.1.37` → `192.168.1.65` (DHCP).
Consequences observed:

- `/etc/hosts` entries (line 5) still point at `.37` → every
  `*.ros-platform.local` URL dead from the browser, even though the
  panel is healthy (`127.0.0.1:8082` → 200). User needs the sudo
  hosts fix again.
- `manifests/control-panel-deployment.yaml` line 81 hardcodes
  `https://192.168.1.37:6443` (join/kubeconfig path). Debt: make the
  IP configurable or drift-proof (DHCP reservation is the cheap fix).

Nothing live depends on the stale IP for this proof (used 127.0.0.1),
but browser-facing UI is unreachable until `/etc/hosts` is corrected.

## Dirty-tree discipline

Committed in this chunk: `csrf.go`, `csrf_test.go`,
`control-panel-deployment.yaml` (comment), this report. The 5
untracked junk files left alone. No push.
