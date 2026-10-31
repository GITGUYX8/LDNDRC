# Checkpoint 42 — R2-final: durable panel DBs + storm-proof rollout/probes

## What landed (manifests + new PVC, all applied live)

| Change | Why |
|---|---|
| `manifests/control-panel-pvc.yaml` (new): 1Gi RWO, `local-path` | Panel DBs must survive pod redeploys, not just container restarts |
| Deployment `volumes.data`: emptyDir → PVC | same; `SESSIONS_DB` + `NODES_DB` both live under `/var/lib/ldndrc` on the PVC |
| `SESSIONS_DB` env now actually live (was missing in-cluster; repo had it, cluster didn't) | R2 session records were still in-memory-only in production |
| `strategy: maxSurge 0 / maxUnavailable 1` | hostNetwork pod binds node `:8082`; a surge pod deadlocks forever (CP-20 §6). This formalizes delete-old-first — future `rollout restart` should behave, but delete-pod stays the habit until proven |
| Probes pinned `host: 127.0.0.1` | **root cause of the restart storm** (below) |
| `K3S_JOIN_URL` → `https://192.168.1.42:6443` | stale `.37` value; IP churn makes this a moving target (DHCP reservation = standing debt) |

## Two root-cause discoveries (both from the same churn family)

1. **141-restart crashloop storm (panel `RestartedCount` 95→101→141 over 3 days).**
   hostNetwork pod's probes default to the podIP — baked in at pod
   creation. The laptop's DHCP lease drifted (`.37` → `.65` → `.42`),
   the baked IP went dead, liveness killed the container, and kubelet
   restarted it *inside the same sandbox with the same stale IP* →
   permanent crashloop. Fix: loopback probes. Post-fix: the identical
   server has run 3d6h+ with **0 restarts**.
2. **Pod-network outage (blocked the PVC bind).** The first apply left
   the new pod Pending: `rancher.io/local-path` was alive but its
   reflector had `dial tcp 10.43.0.1:443: no route to host` — no
   pod-network pod could reach the service ClusterIP (proved with a
   scratch pod + wget). Same stale-flannel/iptables family as #1.
   Fix needed `sudo systemctl restart k3s` (user-run, Oct 27) →
   PVC bound, cluster healthy.

## Durability proof (the R2-final headline)

1. Login as `durability` → `POST /api/sessions` → `202`, SID
   `117fd258af33` (Deployment + Service + 5Gi PVC created, all bound).
2. `kubectl delete pod -l app.kubernetes.io/name=ldndrc-control-panel` —
   the exact operation that used to wipe every session record.
3. Fresh pod ready in ~5s; boot log: `sessions reconcile: 1 adopted,
   0 tombstoned` — records loaded **from the PVC** and the live
   Deployment adopted.
4. `GET /api/sessions` from a fresh login → `117fd258af33` still there,
   `provisioning` → pod shortly `1/1 Running` (real workspace image,
   left running for use).

The memory-hole that motivated R2 is closed: panel redeploys no longer
lose sessions.

## Known cost of the emptyDir → PVC swap

The old pod's `nodes.json` went with the emptyDir. Approved-laptop
registrations from the rehearsal are gone; laptops re-join via the
join TUI (same flow as before). `downloads.json` had no legacy data.

## Side discovery: parallel in-flight work

The worktree holds an uncommitted, complete second chunk from Oct
27–30: the **LAN onboarding portal** (its own CP-41). Tests green, docs
written, not deployed. Committed as its own commit immediately after
this one (manifest's `DOWNLOADS_DB` block was surgically separated so
neither commit carries the other's work). Nothing in that chunk was
modified beyond one manifest comment refresh to match the new PVC
reality (its "emptyDir caveat" text was honest then; the store now
lives on the PVC).

## Deploy state after this chunk

- Cluster = repo manifest (applied Oct 27, verified today): PVC-backed,
  loopback probes, safe strategy, `SESSIONS_DB` + stale join URL fixed.
- Panel image `ldndrc/control-panel:dev` (imported Oct 26) predates the
  portal code — env `DOWNLOADS_DB` is live but dormant until a rebuild.
  Portal deploy = next chunk's rebuild + import + delete-pod rollout.
- Durability session `117fd258af33` left Running for use.

## Discipline

No push. Untracked junk untouched. Image tarballs live in `$HOME`
(`/tmp` proved worthless again — it ate the CSRF tarball Oct 26).
