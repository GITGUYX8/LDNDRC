# Checkpoint 43 — The Front Door, Live on Real Hardware

## The big picture first

Checkpoint 41 built the "front door" of the workshop network: a welcome page with
enrollment scripts and a live download counter, served from the master laptop before
anyone is approved. This checkpoint is the first time that front door stood up on
**real hardware with a real second laptop**: a Windows machine joined the
`ldndrc-onboarding` Wi-Fi, got the captive-style prompt, opened our portal, watched
the counter tick, and downloaded the Windows enrollment script three times — all
recorded by the panel itself.

The same checkpoint also turned the ad-hoc live-test scaffolding into a reproducible
harness. What started as hand-edited files in `~/ldndrc-portal-test/` is now
`tools/captive-portal/`: parameterized, documented, covered by a root-free redirect
test, and honest about which parts need a password.

The full enrollment handshake (script → register → instructor approves → script
says APPROVED) was **not** exercised from that laptop this session. That remains the
next test.

## What got built

### The product (already in the panel)

The portal itself ships inside the control-panel binary, committed as
`383624d feat(portal): LAN onboarding portal + served-download counter`:

| Piece | What it is |
|---|---|
| `/portal` | Welcome page: run instructions, download links, live counter |
| `/portal/joining-cluster.{sh,ps1}` | Enrollment scripts, served with download accounting |
| `/api/downloads/count` | Public aggregate counter (polled by the page every 3 s) |
| `/api/downloads/stats` | JWT-gated counter detail for the instructor console |

### The harness (promoted this checkpoint)

The live rig was originally created outside the repo as throwaway debugging tools.
It is now committed as `7088b7d feat(captive-portal): promote the onboarding AP rig
into tools/captive-portal`, with everything machine-specific turned into a flag or
an environment variable:

| File | Friendly name | What it does |
|---|---|---|
| `tools/captive-portal/run.sh` | The Stage Manager | `start` / `stop` / `status` / `config` / `sudo`: builds the panel, renders configs, starts the unprivileged half, prints the root block. |
| `tools/captive-portal/captive-trap.py` | The Doorman | Answers every intercepted request with a 302 to the portal; derives the portal URL from the hotspot interface (or a fixed `--panel-url`). |
| `tools/captive-portal/ldndrc-captive.nft.tmpl` | The Interceptor | nftables rule: any port-80 packet arriving on the hotspot interface is redirected to the Doorman on `:8090`. |
| `tools/captive-portal/dnsmasq-onboarding.conf.tmpl` | The Clerk | Hands out DHCP leases (`192.168.4.50–150`) and answers **every** hostname with the master's IP. |
| `tools/captive-portal/README.md` | The Manual | Quickstart, configuration, why NM shared mode failed, troubleshooting, safety notes. |
| `test-captive-trap.sh` | The Contract Check | 10 redirect assertions (302, `Location`, `no-store`, fallback body, HEAD/POST, `--once`); no root, ~2 s. |
| `$RUNTIME_DIR/panel` (default `~/.local/state/ldndrc-captive/`) | The Reception Desk | The control-panel binary with the portal feature, run standalone on `:8083` during the test. |

Nothing in the harness writes to `/etc` and no runtime state enters git. The
privileged steps (hotspot, dnsmasq, nftables) are **printed, never executed** — the
operator sees exactly what will change and supplies the password.

Docs updated in the same commit:

- `docs/lan-onboarding-portal.md`: the speculative `hostapd` recipe (never used) is
  replaced by the arrangement that actually worked, including the
  `Operation not permitted` failure that forced it.
- `README.md`: the harness is linked from the onboarding section and the project
  structure.
- `.gitignore`: Python cache from the trap.

This checkpoint's own first commit was `8e75063 docs: checkpoint-43 captive portal
live test`; it also carried a tiny `joining-cluster.ps1` tweak (dropped the 🎉 emoji,
which Windows PowerShell 5.1 consoles print as mojibake).

## How each piece works

### The Hotspot — NetworkManager AP mode on `wlp0s20f3`

Think of it as the master laptop wearing a second hat: it keeps its home Wi-Fi
connection and also broadcasts `ldndrc-onboarding` (WPA2, password
`ldndrc-join-2026`). The Intel card supports both at once on the same channel.

**Watch out for:** NetworkManager's `ipv4.method=shared` helper (its internal
dnsmasq) could not even be *spawned* on this machine — every attempt died with
`Failed to execute child process "/usr/bin/dnsmasq" (Operation not permitted)`,
even after giving dnsmasq file capabilities. The fix that worked: pin the hotspot
to `ipv4.method=manual` with `192.168.4.1/24`, and run our own dnsmasq as root
(no capability tricks involved). Same result, one less moving part.

`run.sh start` prints this arrangement as a copy-paste block; it does not run it.

### The Interceptor — `ldndrc-captive.nft.tmpl`

Every web request that arrives on the hotspot's radio is quietly rewritten to
point at the Doorman. It matches on the **interface**, not a subnet, because
NetworkManager's subnet choice was the flaky part. The template exists so the
interface and trap port come from configuration, not from this laptop's hardware.

### The Doorman — `captive-trap.py`

It answers with `302 Found → http://<hotspot-ip>:<panel-port>/portal`, which is
exactly what browsers and operating systems understand as "this network wants you
to sign in". The IP is re-read on every request, so it follows the hotspot wherever
it lands; `--panel-url` pins it for tests and non-hotspot setups.

### The Clerk — `dnsmasq-onboarding.conf.tmpl`

DHCP plus a wildcard DNS answer (`address=/#/192.168.4.1`): any name a student
types resolves to the master, so even ad-hoc browsing ends up at our portal. Runs
as root directly, bypassing NM's broken helper path entirely.

### The Stage Manager — `run.sh`

One entry point for the whole rig:

```bash
./tools/captive-portal/run.sh start    # build + start panel & trap, print root block
./tools/captive-portal/run.sh status   # panel / trap / hotspot / iface / counter / portal URL
./tools/captive-portal/run.sh sudo     # just the root block again
./tools/captive-portal/run.sh stop     # stop panel & trap; print root teardown
./tools/captive-portal/run.sh config   # resolved iface, IP, ports, runtime dir
```

It refuses to start on an occupied panel port (with a hint to use `PANEL_PORT=8083`
when a deployed panel pod owns `:8082`), waits for `/healthz` and the trap's 302
before declaring success, and keeps every file under `$RUNTIME_DIR`.

### The Reception Desk — the panel on `:8083`

The real portal page, run from the standalone binary. Its counter polls
`GET /api/downloads/count` every three seconds and updates in place — no reload.
Counting stays honest: a "download" is a full served body; previews, retries and
range probes are requests only.

## How it all connects

```
./run.sh start
  → builds panel, renders nft + dnsmasq configs, starts panel :<port> and trap
  → prints the root block (hotspot up, dnsmasq, nft)
        ↓ (operator runs that block)
Windows laptop joins "ldndrc-onboarding"
  → Clerk gives it 192.168.4.x and "all names = master"
  → OS probe / typed URL reaches port 80
  → Interceptor redirects to Doorman :8090
  → Doorman 302s to the Reception Desk :<panel-port>/portal
  → student downloads joining-cluster.ps1 (counter ticks live)
```

## Proof it works

### The live session (Nov 18)

Actual evidence from master-side logs and on-disk counters:

- `panel.log`: the portal page was served and its counter **polled every 3
  seconds** from 16:35 to 16:50 — a browser had the portal open for ~15 minutes
  (later polls at ~1-minute spacing show the tab backgrounded/throttled).
- `panel.log`: `GET /portal/joining-cluster.ps1` at **16:39:40, 16:39:47 and
  16:41:40** — three downloads from the joining laptop.
- `run-2/downloads.json`: `requests: 3, downloads: 3`, attributed to **one hashed
  device** — the counter's semantics behaving exactly as designed.
- User-observed: the joining **Windows laptop** showed the captive-style prompt on
  its own; it needed a click, then opened the portal.

### The promoted harness (this checkpoint)

Run after the promotion, on this machine:

| Check | Result |
|---|---|
| `go vet ./...` + `go test -count=1 ./...` (control-panel) | all packages ok |
| `./test-joining.sh` | 5 passed, 0 failed |
| `./test-captive-trap.sh` | 10 passed, 0 failed |
| `run.sh start` (temp runtime dir, `PANEL_PORT=8083`) | panel healthy, trap answering 302 |
| `run.sh status` | panel up, trap 302, portal 200, counter API returned aggregate totals |
| `run.sh stop` | both processes stopped cleanly; root teardown printed |

**Not proven this session:** the enrollment loop from a real second laptop
(script registers → instructor approves in the console → script prints APPROVED).
That loop was proven earlier from the master's own shell (approve → exit 0,
deny → exit 1 with reason, bad JOIN_KEY → immediate rejection, re-run → same node
id), but not yet driven from the Windows machine.

**Caveat:** `trap.log` from the Nov 18 session is empty (the trap wasn't logging to
that file during the successful run), so we can't attribute the prompt to the nft
interceptor vs the OS's own captive detection. The promoted trap logs every
intercepted request, so the next session settles it.

## The one-liner version

| Piece | One-liner |
|---|---|
| `run.sh` | Brings up the panel + trap and prints the root steps; `status` says what is alive. |
| `captive-trap.py` | Sends every intercepted request to the portal with a 302. |
| `ldndrc-captive.nft.tmpl` | Turns the hotspot's port 80 into a funnel. |
| `dnsmasq-onboarding.conf.tmpl` | Gives laptops addresses and makes every name the master. |
| `panel :8083` | The welcome page, scripts and live counter as a plain web page. |
| `test-captive-trap.sh` | Proves the redirect contract in two seconds without root. |

## What's next

- [ ] Drive the full enrollment loop from the Windows laptop (download → run →
      instructor approves → `APPROVED`), now that the portal is reachable.
- [ ] Apply the four pending `joining-cluster.ps1` fixes: 503-is-approved
      handling, real status on registration, PS7-safe 403 detection,
      `TEST_HOSTNAME`.
- [ ] Re-run one session with the Doorman logging so the captive prompt path is
      captured, not inferred.
- [ ] Cut the feature into the real deployment: the old pod image still owns
      `:8082` (that's why this test used `:8083`).
- [ ] Cleanup/restart cheatsheet:
      `./tools/captive-portal/run.sh stop`, then
      `sudo nft delete table ip ldndrc-captive`,
      `sudo kill "$(cat ~/.local/state/ldndrc-captive/dnsmasq.pid)"`,
      `nmcli connection down ldndrc-onboarding`.
