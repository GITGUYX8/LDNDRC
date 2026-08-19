# Checkpoint 01 — Join Logic & k3d Cluster Tests

## The big picture first

Your project wants to turn a pile of old laptops into a shared robot-simulation
farm: strong laptops run the heavy simulators, weak laptops just open a browser
and watch. The problem — you only have one laptop right now. So this checkpoint
built two things: a "gatekeeper" script that will eventually decide what each
real laptop is allowed to do, and a **fake miniature version of the whole farm**
inside Docker containers, so every rule could be rehearsed before real hardware
shows up.

## What got built

| File | Friendly name | What it does |
|---|---|---|
| `join-cluster.sh` | The Bouncer | Asks a laptop "are you strong enough?" and lets it join the farm only if yes |
| `test-join.sh` | The Bouncer's Exam | Feeds the Bouncer 4 fake laptops and checks it makes the right call each time |
| `manifests/test-pod.yaml` | The Test Passengers | Tiny placeholder workloads that must only sit on "Host" laptops |
| `manifests/test-svc.yaml` | The Mailbox | One stable address so others can always find the Test Passengers |
| `manifests/test-ingress.yaml` | The Front Door Sign | Tells browsers "traffic for `/sim` goes to the Mailbox" |

## How each piece works

### The Bouncer — `join-cluster.sh`

Every real laptop will eventually run this script. It checks three things:
at least 8 CPU cores (the chip's worker count), at least 16 GB of RAM
(short-term memory), and an NVIDIA graphics card. All three must be yes, and
the laptop joins the farm wearing a "Host" badge. Any single "no" and the
script politely says "just use your browser instead."

Since there's only one real laptop, the script accepts pretend answers
(`TEST_CPU=4` and friends) so you can rehearse both outcomes without owning
more machines.

### The Bouncer's Exam — `test-join.sh`

Runs the Bouncer against 4 imaginary laptops — a strong one (gets in), a weak
one (turned away), and two sneaky cases that pass two checks but fail one
(GPU missing; CPU too weak). Those sneaky cases prove the Bouncer really does
demand *all three*, not just most of them.

### The Test Passengers, Mailbox & Front Door — `manifests/`

Three small rehearsal pieces for the farm itself. The Test Passengers carry a
rule: "seat us only on Host-badged machines." The Mailbox is a permanent
address that always points at them, even as they come and go. The Front Door
Sign says "anyone asking for `/sim` gets shown to the Mailbox" — exactly how a
guest browser will one day reach the real simulation screen.

One fix mattered here: the Passengers originally just slept, which meant the
Front Door had a sign pointing at a door nobody answered. They now run a tiny
one-line web page, so the full browser-to-workload path can be proven.

## How it all connects

```
A laptop runs the Bouncer
        │
        ├─ strong enough ─► joins the fake farm (Docker containers
        │                   pretending to be laptops) with a "Host" badge
        │
        ▼
Test Passengers ask for Host-badged seats only
        │
        ▼
A browser asks for ros-platform.local/sim
        │
        ▼
Front Door Sign ─► Mailbox ─► Test Passenger (sitting on a Host machine)
```

## Proof it works

- **Bouncer's Exam:** 4 out of 4 scenarios judged correctly (all PASS).
- **Joining:** a new container joined the fake farm and showed up wearing the
  Host badge, exactly as the Bouncer decided.
- **Seating rule:** both Test Passengers landed only on the Host-badged
  machine — never on a Guest or the manager node.
- **Losing a machine:** two worker containers were forcibly stopped; the farm
  noticed within about a minute, marked them gone, and moved their work onto a
  healthy machine automatically.
- **The browser path:** asking for `http://ros-platform.local/sim` now returns
  a real page (HTTP 200) from a Passenger sitting on the Host machine — the
  exact route a guest's browser will take to reach a simulation.

## The one-liner version

| Piece | One-liner |
|---|---|
| `join-cluster.sh` | Only strong laptops may join; weak ones are told to browse |
| `test-join.sh` | Proves the Bouncer enforces all three requirements, 4/4 |
| `manifests/` | Prove workloads go to the right machines and browsers can reach them |

## What's next

- [ ] Build the real simulation container (ROS2 Jazzy + Gazebo Harmonic with
      the browser-view plugin)
- [ ] Write the real farm manifest (the StatefulSet that gives every session
      its own private radio channel, `ROS_DOMAIN_ID`)
- [ ] Rehearse the same browser path against the real simulation view instead
      of the placeholder page
- [ ] Eventually, run the Bouncer on a second real laptop
