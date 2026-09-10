# Checkpoint 16 — The Vending Machine (Release Pipeline)

## The big picture first

The join program worked on our machine, but the second laptop couldn't reach
it — passing binaries around on USB sticks doesn't scale past one workshop.
This checkpoint built a vending machine: tag a version, and GitHub
automatically builds the program for every laptop flavor, checks them, and
puts them on a shelf with tamper-evident seals.

## What got built

| File | Friendly name | What it does |
|---|---|---|
| `.github/workflows/release-join.yml` | The Vending Machine | On every version tag, builds 5 binaries, checksums them, publishes the release |
| `docs/host-joiner-tui-spec.md` (distribution section) | The Shopping List | Tells laptop owners exactly what to download and how to verify it |

## How each piece works

### The Vending Machine — `.github/workflows/release-join.yml`

Think of it as a factory that wakes up whenever we stamp a version number
like `v0.1.0`. It builds the join program five ways — Linux, Windows, and
Mac, on both common chip types — runs all the tests first (a broken build
never ships), then seals each binary with a fingerprint (a SHA256 checksum)
so downloaders can prove nobody tampered with it. We skipped the fancy
packaging robot (GoReleaser) on purpose: plain workshop tools, fewer things
that can break, zero cost.

### The Shopping List — TUI spec distribution section

Three copy-paste lines for the second laptop: download, verify the seal,
run. We learned one real lesson here the honest way — renaming the download
breaks the seal check, so the instructions now keep the release filename.
That bug was caught by our own verification, not by a user.

## How it all connects

1. We merge finished work to `main`.
2. We stamp a tag (`git tag v0.1.0 && git push origin v0.1.0`).
3. The factory builds, tests, seals, and shelves all five flavors (~2 min).
4. Any laptop downloads its flavor, checks the seal, runs `--plain`.
5. No USB sticks, no `scp`, no "which version do you have?" confusion.

## Proof it works

Here's what actually happened:

- **First push bounced** — GitHub refused the workflow file because our login
  token lacked the `workflow` permission. Fixed with `gh auth refresh`,
  retried, went through.
- **All 5 builds green + release published** in about 2 minutes (the Node.js
  warnings in the log are harmless noise from GitHub's side).
- **Fresh download, seal check OK**, and the release binary printed the Host
  verdict under test overrides — plus the predicted Guest verdict on this
  laptop's real hardware (15 GB under the 16 GB bar), exit 0.
- **Second-laptop handoff ready**: four copy-paste lines, checksums included.

## The one-liner version

| Piece | One-liner |
|---|---|
| `release-join.yml` | Stamp a version, get five sealed binaries for free. |
| Distribution docs | Download, verify the seal, run — no USB sticks. |

## What's next

- [ ] Push the local docs commit (`db8885c`) — no permission issues expected.
- [ ] Run the second-laptop acceptance: download, `--plain`, TUI matrix.
- [ ] P2 mDNS advertisement so `MASTER_IP` typing goes away.
- [ ] Wire the TUI Join screen to the engine step runner.
