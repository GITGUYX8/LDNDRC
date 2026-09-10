# Checkpoint 15 — The Join Binary: Engine, Dashboard, and Black Box

## The big picture first

Until now, inviting a laptop into the cluster meant typing secret tokens by
hand and hoping. This checkpoint built the other half of that handshake: a
small program called `ldndrc-join` that runs on the *joining* laptop. It
checks whether the machine is strong enough, phones home to ask permission,
waits politely for a human to approve, then installs everything itself.

Think of it as a self-driving guest pass. The laptop proves it belongs, asks
at the door, and only comes in when someone says yes.

## What got built

| File | Friendly name | What it does |
|---|---|---|
| `internal/join/hardware.go` | The Rulebook | The strength bars (8 cores, 16 GB, NVIDIA, driver 535+) and the verdict: Host or Guest, no exceptions |
| `internal/join/detect_linux.go` | The Inspector | Measures the real laptop: chip count, memory, graphics card, disk space, admin rights |
| `internal/join/detect_windows.go` | The Windows Doorman | Measures the physical machine (not the virtual one inside it) |
| `internal/join/detect_darwin.go` | The Mac Doorman | Macs are always Guests — it just hands them the browser address |
| `internal/join/discover.go` | The Bloodhound | Sniffs out the master laptop on the local network by itself |
| `internal/join/register.go` | The Phone Call | Registers with the master, then waits on hold for approval (checks every 5 seconds, hangs up after 15 minutes) |
| `internal/join/install_linux.go` | The Mechanic | Installs the graphics toolkit and the cluster agent — but only with explicit permission, and never writes passwords anywhere visible |
| `internal/join/diagnostics.go` | The Black Box | Writes a flight-recorder file of what happened; sends it home only if the network proved healthy |
| `internal/join/model.go` | The Dashboard | The four screens you see: Checks, Fix-it card, Joining, Done |
| `cmd/join/main.go` | The Front Door | Opens either the full screen experience or a plain log mode for scripts |
| `internal/httpapi` (diagnostics route) | The Inbox Slot | The master's new mailbox where laptops can drop their flight-recorder files (1 MB limit) |

## How each piece works

### The Rulebook — `hardware.go`

Like the height chart at a roller coaster: you must be *this* tall to ride.
Eight processor cores, 16 GB of memory, an NVIDIA graphics card with a
recent driver, 25 GB of free disk, admin rights, and a reachable master.
Fail any single one and you're kindly shown to the browser instead. We
tested it against 9 different fake laptops — strong ones, weak ones, and
sneaky ones that pass everything but one check.

### The Inspector — `detect_linux.go`

This is the part that looks under the hood. It reads the chip count, parses
the memory file, asks the graphics card who it is *and* whether its driver
actually works (just existing isn't enough — that's an old trap we
documented). It also accepts pretend answers (`TEST_CPU=12` and friends) so
we can rehearse without owning ten laptops.

### The Doormen — `detect_windows.go` + `detect_darwin.go`

The Windows doorman measures the *physical* laptop, not the virtual machine
hiding inside it — otherwise a weak laptop could disguise itself. The Mac
doorman has the easiest job: every Mac is a Guest, so it just hands over
the browser address and wishes you a nice day.

### The Bloodhound — `discover.go`

Nobody wants to type IP addresses. The bloodhound sniffs the local network
for the master's announcement and finds it alone in about 3 seconds. If the
network blocks sniffing (some managed Wi-Fi does), you can still type the
address by hand as a fallback.

### The Phone Call — `register.go`

Registers the laptop ("hi, I'm Priya's machine, here's what I have") and
gets back an ID. Then it calls back every 5 seconds: still waiting? Still
waiting? Approved? The secret token it eventually receives lives only in
memory — never saved to disk, never printed. When approval finally arrives,
it also confirms the master is reachable before celebrating.

### The Mechanic — `install_linux.go`

Installs the graphics toolkit and the cluster agent wearing the Host badge.
Two strict rules: it never runs silently (you always say yes first), and
the secret token never appears in any log or error message. We proved the
token-redaction with tests.

### The Black Box — `diagnostics.go`

Whatever happens — success or failure — a flight-recorder file is written
locally first, so there's always something to show the instructor. If the
approval calls were going through (meaning the network is healthy), the
file is also sent home automatically. If the network itself is suspect, it
honestly says so and tells you the exact filename to share by hand.

### The Dashboard — `model.go`

Four screens, keyboard-driven: the checklist, the fix-it card for whatever
turned red, the live join log, and the finish screen. We tested every
keypress transition without needing a real terminal — press `j` with a red
row and it correctly refuses to join.

### The Front Door — `cmd/join/main.go`

Two entrances: the full-screen experience, or `--plain` mode that prints
the exact same two log lines the old shell script printed — byte for byte,
so all existing tests keep passing.

## How it all connects

1. You run `ldndrc-join` on a laptop.
2. The Inspector measures the machine; the Rulebook pronounces Host or Guest.
3. Guests get the browser address; Hosts continue.
4. The Bloodhound finds the master; the Phone Call registers and waits.
5. A human approves on the dashboard (or via API for now).
6. The secret token arrives; the Mechanic installs everything.
7. The Black Box records the whole story and sends it home if it can.
8. Done screen: success message, useful links, how to leave the cluster.

## Proof it works

Here's what actually happened when we ran it:

- **Go tests, all packages green** — rulebook gates, driver colors, phone-call
  flow against a fake master, dashboard keypresses, upload round-trips.
- **`--plain` output, byte-for-byte** — all 4 hardware scenarios print
  exactly the lines the old shell tests grep for, and the old `test-join.sh`
  still passes 4/4.
- **Three operating systems, one codebase** — Linux, Windows, and Mac builds
  all compile into ~7.5 MB static binaries with no installation needed.
- **Live cluster check** — the rebuilt control panel accepted a real
  diagnostics upload (`received`), and the test laptop's request was
  tidily denied afterwards.

## The one-liner version

| Piece | One-liner |
|---|---|
| `hardware.go` | The height chart: tall enough to ride, or enjoy the browser. |
| `detect_*.go` | Three doormen who actually measure you (Macs get waved to Guest). |
| `discover.go` | Finds the master by sniffing, no typing needed. |
| `register.go` | Phones home, waits on hold, guards the secret token with its life. |
| `install_linux.go` | The mechanic who asks permission and hides the passwords. |
| `diagnostics.go` | The flight recorder that phones home only when the line is good. |
| `model.go` + `main.go` | Four pretty screens, or plain logs for the robots. |

## What's next

- [ ] Connect the Join screen to the real engine runner (register → poll →
      consent → install → verify) end to end.
- [ ] P2: master announces itself on the network (the bloodhound needs
      something to sniff).
- [ ] Real-world acceptance: a second physical laptop joins a real master.
- [ ] Commit this checkpoint's changes when ready.
