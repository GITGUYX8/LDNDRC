# Checkpoint 43 — The Front Door, Live on Real Hardware

## The big picture first

Checkpoint 41 built the "front door" of the workshop network: a welcome page with
enrollment scripts and a live download counter, served from the master laptop before
anyone is approved. This checkpoint is the first time that front door stood up on
**real hardware with a real second laptop**: a Windows machine joined the
`ldndrc-onboarding` Wi-Fi, got the captive-style prompt, opened our portal, watched
the counter tick, and downloaded the Windows enrollment script three times — all
recorded by the panel itself.

The full enrollment handshake (script → register → instructor approves → script
says APPROVED) was **not** exercised from that laptop this session. That remains the
next test.

## What got built

The test rig (deliberately outside the repo — it is a harness, not product):

| File | Friendly name | What it does |
|---|---|---|
| `~/ldndrc-portal-test/captive-trap.py` | The Doorman | Answers every intercepted web request with a 302 to the portal; derives the portal URL from the hotspot's own IP. |
| `~/ldndrc-portal-test/ldndrc-captive.nft` | The Interceptor | nftables rule: any port-80 packet arriving on the hotspot interface gets redirected to the Doorman on `:8090`. |
| `~/ldndrc-portal-test/dnsmasq-onboarding.conf` | The Clerk | Hands out DHCP leases (`192.168.4.50–150`) and answers **every** hostname with the master's IP. |
| `~/ldndrc-portal-test/panel` | The Reception Desk | The new control-panel binary: portal page, script downloads, live counter (ran standalone on `:8083`). |

Repo-side, this checkpoint also carries the portal feature commit
(`383624d feat(portal): LAN onboarding portal + served-download counter`) and a tiny
tweak to `joining-cluster.ps1` (dropped the 🎉 emoji, which Windows PowerShell 5.1
consoles print as mojibake).

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

### The Interceptor — `ldndrc-captive.nft`

Every web request that arrives on the hotspot's radio is quietly rewritten to
point at the Doorman. It matches on the **interface**, not a subnet, because
NetworkManager's subnet choice was the flaky part.

### The Doorman — `captive-trap.py`

It answers with `302 Found → http://<hotspot-ip>:8083/portal`, which is exactly
what browsers and operating systems understand as "this network wants you to sign
in". The IP is re-read on every request, so it follows the hotspot wherever it
lands.

### The Clerk — `dnsmasq-onboarding.conf`

DHCP plus a wildcard DNS answer (`address=/#/192.168.4.1`): any name a student
types resolves to the master, so even ad-hoc browsing ends up at our portal.

### The Reception Desk — the panel on `:8083`

The real portal page. Its counter polls `GET /api/downloads/count` every three
seconds and updates in place — no reload. Counting stays honest: a "download" is a
full served body; previews, retries and range probes are requests only.

## How it all connects

```
Windows laptop joins "ldndrc-onboarding"
  → Clerk gives it 192.168.4.x and "all names = master"
  → OS probe / typed URL reaches port 80
  → Interceptor redirects to Doorman :8090
  → Doorman 302s to the Reception Desk :8083/portal
  → student downloads joining-cluster.ps1 (counter ticks live)
```

## Proof it works

Actual evidence from the Nov 18 session (master-side logs and on-disk counters):

- `panel.log`: the portal page was served and its counter **polled every 3
  seconds** from 16:35 to 16:50 — a browser had the portal open for ~15 minutes
  (later polls at ~1-minute spacing show the tab backgrounded/throttled).
- `panel.log`: `GET /portal/joining-cluster.ps1` at **16:39:40, 16:39:47 and
  16:41:40** — three downloads from the joining laptop.
- `run-2/downloads.json`: `requests: 3, downloads: 3`, attributed to **one hashed
  device** — the counter's semantics behaving exactly as designed.
- User-observed: the joining **Windows laptop** showed the captive-style prompt on
  its own; it needed a click, then opened the portal.

**Not proven this session:** the enrollment loop from a real second laptop
(script registers → instructor approves in the console → script prints APPROVED).
That loop was proven earlier from the master's own shell (approve → exit 0,
deny → exit 1 with reason, bad JOIN_KEY → immediate rejection, re-run → same node
id), but not yet driven from the Windows machine.

**Caveat:** `trap.log` from this session is empty (the trap wasn't logging to that
file during the successful run), so we can't attribute the prompt to the nft
interceptor vs the OS's own captive detection. Re-running with the Doorman logging
will settle it.

## The one-liner version

| Piece | One-liner |
|---|---|
| `captive-trap.py` | Sends every intercepted request to the portal with a 302. |
| `ldndrc-captive.nft` | Turns the hotspot's port 80 into a funnel. |
| `dnsmasq-onboarding.conf` | Gives laptops addresses and makes every name the master. |
| `panel :8083` | The welcome page, scripts and live counter as a plain web page. |

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
- [ ] Cleanup/restart cheatsheet: `kill` panel+trap, `sudo nft delete table ip
      ldndrc-captive`, `sudo kill $(cat /run/dnsmasq-ldndrc-onboarding.pid)`,
      `nmcli connection down ldndrc-onboarding`.
