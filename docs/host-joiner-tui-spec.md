# Host Joiner TUI Spec (`ldndrc-join`)

Reference spec for the laptop-side onboarding client: a single static Go
binary with a keyboard-driven terminal UI (Bubble Tea). Companion to
`onboarding-design-report.md` (protocol) and checkpoint 14 (nodes API, P1).
This document locks the UX decisions agreed 2026-09-06.

## Decisions

- **Interface: terminal UI (option C).** No Fyne/Wails/Electron. Reasons:
  single static binary, zero signing pain on Linux, works over SSH, tiny
  dependency footprint (`bubbletea`, `bubbles`, `lipgloss` — pure Go, no cgo).
- **Zero budget: no code-signing certificates, ever.** Windows SmartScreen
  is handled with a documented "More info → Run anyway" step plus published
  SHA256 checksums. macOS ships a guest-only binary (runs, prints the
  browser URL, installs nothing, exits 0) — no agent/toolkit paths there.
- **Engine/UI split:** all detection and install logic lives in testable
  non-UI functions. `model.go`/`view.go` render state and forward keypresses
  only — they must not contain detection or install logic.
- **Two modes:** default TUI + `--plain` (byte-for-byte the
  `test-join.sh`-grepped lines below, for scripts/CI):
  - `[join-cluster] Detected: CPU=<n> cores, RAM=<n>GB, GPU=yes|no`
  - `[join-cluster] High-end laptop detected. Joining as HOST.`
  - `[join-cluster] Low-end laptop detected. Do not join cluster. Use browser to access platform.`
  `--plain` applies hardware gates only (CPU/RAM/GPU/driver), matching
  `join-cluster.sh`; disk/sudo/network are TUI-mode rows.

### Locked decisions (cited as D1–D4 below)

- **D1 — strictness, no override.** A red blocking row ends the Host path.
  No "continue anyway".
- **D2 — driver rule.** Floor 535 (blocking), amber 535–549 ("works, 550+
  recommended" + upgrade command), green ≥550. `lspci` proves existence
  only — the fix card says so.
- **D3 — diagnostics vs network health.** Approval-poll loop is the network
  probe: polls succeeding → auto-upload the local bundle; failing → skip
  with the manual-share message and exact filename.
- **D4 — download consent.** Nothing downloads before explicit Download
  activation (`enter`); install needs a second confirmation. Same rule
  covers the NVIDIA toolkit install (consent callback, never silent).

## Distribution (the chicken-and-egg)

The binary cannot be downloaded from a master it hasn't discovered yet.
v1 channels: GitHub Release artifacts (`ldndrc-join-linux`,
`ldndrc-join.exe` + checksums) and workshop USB sticks. No installer —
the joiner itself needs zero installation; it runs from anywhere.

## Pre-flight check matrix (Screen 1, read-only)

Nothing is installed or changed during checks. Join stays locked until all
blocking rows are green.

| Row | Probe | Green | Amber (warn, non-blocking) | Red (blocking) |
|---|---|---|---|---|
| OS | `runtime.GOOS` + version | recent Linux; Win 10/11 (WSL2 path) | — | macOS → Guest ending (no K3s/NVIDIA possible) |
| CPU | core count | ≥ 8 | — | < 8 → Guest |
| RAM | total physical RAM | ≥ 16 GB | — | < 16 GB → Guest |
| GPU | `nvidia-smi -L` | NVIDIA present | — | absent → Guest |
| Driver | `nvidia-smi` version parse | ≥ 550 | 535–549 ("works, 550+ recommended" + upgrade cmd) | < 535 or missing. Note: `lspci` proves existence only — say so on the fix card |
| Toolkit | `nvidia-container-toolkit --version` | present | — | missing but installable via apt (offered later with consent) |
| Disk | free space on `/` | ≥ 25 GB (join ~1.5 GB + first workload pull ~5–10 GB + headroom) | — | below → Guest/stop |
| Network | TCP dial master:6443 | reachable | — | fail → show exact `ufw allow` lines |
| Sudo | can-elevate check | yes | — | no → stop (agent + toolkit install need it) |
| Master | address set (typed now, mDNS-filled later) | set | — | unset → Join locked |

Failing any blocking gate ends the Host path on Screen 1: browser URL,
nothing installed, exit 0. **No "continue anyway" override — strictness is
mandatory (D1).**

## Driver rule detail (D2)

- Floor **535**: the branch where container-toolkit time-slicing and the
  workspace image's CUDA needs are reliably supported. Below it, GPU sharing
  silently misbehaves — hence blocking, not advisory.
- Recommended **550+**: aging-out-of-support window; newer toolkit releases
  assume newer branches.
- Fix card shows: detected version (or "none"), floor, recommendation,
  exact distro install command, Secure-Boot/MOK warning (module installed
  but not loaded → enroll key or disable Secure Boot), `r` to re-run the
  single check. Informational only — never executes (see consent rule).

## Screens

1. **Checks** — matrix above; Guest ending here for unqualified machines.
2. **Fix guidance** — focused red row → fix card (meaning + exact command +
   re-run). Drivers: info only, acquisition is gated per consent rule.
3. **Join** — master field, Join keypress, ordered step log:
   register → approval wait → short-lived token (never on disk, never shown
   in full) → toolkit install if consented → K3s agent install with
   `role=host` → verify Ready + labelled → success + URLs + leave
   instructions. Approval wait: 5s poll, 15-min timeout, clean
   denied/timeout paths.
4. **Done / Diagnosed** — success screen, or failing step + exact error +
   fix command + `d` to write `ldndrc-diagnostics-<timestamp>.txt`
   (OS, check outputs, step log; tokens/secrets excluded by construction).

### Windows handoff (report §7.2 — not re-specified here)

Detection runs natively via Win APIs so thresholds measure the physical
laptop, not the VM. On qualify: if WSL2 is present (`wsl --status`), the
binary copies the Linux build into the WSL filesystem and re-runs steps
3–9 inside it. If WSL2 is absent: guided steps only (`wsl --install`,
reboot, mirrored networking on Win 11 22H2+ so Flannel VXLAN UDP 8472 is
reachable inbound) — no silent auto-install. If mirrored networking is
unavailable: graceful Guest-mode fallback with explanation.

## Diagnostics upload vs network health (D3)

- Local bundle file is **always** written first — the manual path never
  depends on the network.
- The approval-poll loop doubles as the network health probe: polls
  succeeding → network proven healthy → auto-upload the bundle to the
  master without asking.
- Polls failing/timing out → network itself is suspect → **skip**
  auto-upload and say so plainly: "couldn't reach the master, so the
  report wasn't sent automatically — share this file with your instructor
  manually," with the exact filename.
- Status: locked 2026-09-06. Approval polling is the accepted network-health
  signal; mirrored in the onboarding report (§7.1a, §9).

## Driver download consent (D4)

Drivers are never downloaded or installed silently. Ordered consents:

1. Detect → show driver info (what's there, what's needed, package name,
   approximate size).
2. Explicit **Download** activation (`enter`) — nothing downloads before it.
3. Download completes → second explicit confirmation gates **Install**,
   which runs visibly in the step log.

## Developer deliverables

```
control-panel/cmd/join/
  main.go              — flags (--plain vs TUI), launches model
  internal/join/
    hardware.go        — thresholds (pure, unit-tested)
    detect_*.go        — per-OS detection (linux/windows/darwin)
    model.go           — Bubble Tea model: states, messages, transitions
    view.go            — Lip Gloss rendering per screen
    actions.go         — thin calls into engine steps (register/poll/install)
```

## Testing

- Engine: fake CPU/RAM/GPU inputs, fake HTTP server for register/poll
  (existing `test-join.sh` pattern carries over).
- TUI: model-transition tests (keypress in state → expected state; no
  terminal needed) + golden snapshots of the check screen.
- No screenshot testing (out of scope for zero budget).
- Acceptance: one real Linux laptop joining a real master end to end.

## Open items

- [x] Approval-polling as the network-health signal (D3, locked 2026-09-06).
- [ ] Windows SmartScreen-bypass + checksum-verify docs.
- [ ] `GOOS=windows` compile check in CI.
- [ ] P2 mDNS to pre-fill the master field (server side first).
