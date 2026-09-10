# Checkpoint 18 — Join Screen Wired to the Engine

## The big picture first

The join program could check hardware and show pretty screens, but pressing
`j` led nowhere — the dashboard wasn't connected to the engine. This
checkpoint wired them together: the Join screen now runs the full
register → approve → install → verify flow in the background while step
lines stream into the log, plus a consent gate for the toolkit install and
check-matrix builder that turns raw hardware into red/green rows.

## What got built

| File | Friendly name | What it does |
|---|---|---|
| `internal/join/runner.go` | The Tour Guide | Runs the whole trip (register, wait, install, confirm, diagnostics) and narrates each step; token never logged |
| `internal/join/checks.go` | The Report Card | Turns detected hardware into green/red rows with fix-it hints (including the `lspci` lesson and `ufw` lines) |
| `internal/join/install_other.go` | The Bouncer's Cousin | Honest "Linux-only" errors on Windows/Mac so cross-compiles never break |
| `cmd/join/main.go` | The Ignition | Real detection → rows → model; launches the engine exactly once on Join |
| `internal/join/model.go` | The Permission Slip | New consent screen: toolkit installs only on an explicit `y` |

## How it was verified

- `gofmt`/`vet` (linux + windows + darwin) clean; `go test ./...` all `ok`
  (happy-path/denied/timeout runner flows, token-leak assertions on bundle
  and final message, confirm accept/decline/skip, red-row guidance).
- Builds on all three OSes; `--plain` still byte-for-byte; `test-join.sh` 4/4.

## The one-liner version

| Piece | One-liner |
|---|---|
| Runner | The tour guide that narrates the trip and hides the passwords. |
| Consent screen | The toolkit asks first, always. |
| Cross-platform stubs | Windows and Mac builds compile because "no" is a valid answer. |

## What's next

- [ ] Second-laptop acceptance (register → approve → agent install live).
- [ ] P2 mDNS already built; verify announcement seen from second laptop.
- [ ] Commit this checkpoint's changes when ready.
