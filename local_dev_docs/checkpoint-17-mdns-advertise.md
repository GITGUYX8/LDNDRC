# Checkpoint 17 — mDNS Master Advertisement (P2)

## The big picture first

Joining laptops shouldn't have to type the master's IP address — the master
should announce itself on the local network, like a shop hanging its sign
outside. This checkpoint added that announcement: the control panel now
advertises `_ldndrc-master._tcp` via mDNS, which is exactly what the join
binary's bloodhound already knows how to sniff for.

One honest limitation: a normal pod cannot reach LAN radio waves (multicast
never leaves its network bubble), so real masters need a one-file network
patch. The demo cluster stays quiet on purpose.

## What got built

| File | Friendly name | What it does |
|---|---|---|
| `internal/discovery/advertise.go` | The Town Crier | Announces the master (name, port, TXT details) on the LAN; shuts down cleanly |
| `cmd/server/main.go` | The Light Switch | Turns the crier on/off via `ADVERTISE_MDNS` (default on); a failure only logs — the control plane never crashes over announcements |
| `manifests/control-panel-hostnetwork-patch.yaml` | The Balcony Pass | Optional one-file patch letting the pod use the host network on real masters, so announcements reach the LAN |
| `manifests/control-panel-deployment.yaml` | The Quiet Demo | Sets `ADVERTISE_MDNS=false` for k3d (no LAN multicast from a pod netns) |
| `docs/local-cluster-headlamp.md` | The Fixed Manual | Port-forward survival lessons (direct binary + `setsid`) and the real-master mDNS recipe |

## How it was verified

- `TestTXTRecords`: announcement payload contains cluster, version, path.
- `TestAdvertiseAndBrowse`: a real announce-then-find round trip over
  multicast (skips gracefully where sandboxes lack it) — passed here.
- `gofmt`/`vet`/`go test` all green; rebuilt image rolled out to k3d with
  no announcement noise in the logs (as configured).
- Gateway re-verified after rollout: fresh session → editor 200; orphaned
  pre-restart session resources cleaned (in-memory store limits, as known).
- Full LAN proof (second laptop sees the announcement) needs a real master
  with the balcony pass applied — flagged, not faked.

## The one-liner version

| Piece | One-liner |
|---|---|
| Advertiser | The master hangs its sign outside the shop. |
| hostNetwork patch | The balcony pass that lets the sign be seen from the street. |
| Demo default off | The demo stays quiet instead of shouting into a bubble. |

## What's next

- [ ] Second-laptop acceptance: see the announcement, register, join.
- [ ] Wire the TUI Join screen to the engine step runner.
- [ ] Commit this checkpoint's changes when ready.
