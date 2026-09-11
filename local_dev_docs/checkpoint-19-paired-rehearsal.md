# Checkpoint 19 — Paired Register Rehearsal (detailed)

## Context

First laptop-to-master onboarding traffic between two physical machines on
one LAN. Prior onboarding verification was loopback-only (checkpoint-14) or
single-machine (unit/fake-server suites). Scope was checks + register/poll:
no agent install, no sudo on either machine, no mDNS (P2 unproven on LAN —
`MASTER_IP` filled that role).

- Master: this laptop, `192.168.1.37`, k3d `ldndrc` cluster, control panel
  Deployment serving via Traefik `:80`.
- Guest: second laptop, Linux, below Host bar (real specs below).
- `K3S_JOIN_URL` intentionally unset: no joining laptop exists to consume a
  token, so the mint-then-503 path is the expected proof (not a failure).

## Step-by-step with observed outputs

### 1. Register (guest → master, via Traefik)

```bash
curl -s -X POST http://192.168.1.37:80/api/nodes/register \
  -H "Host: control.ros-platform.local" \
  -H 'Content-Type: application/json' \
  -d '{"hostname":"hari","os":"linux","arch":"amd64","cpu":12,"ram_gb":7}'
```

Response:

```json
{"id":"06f5a0d9","status":"pending"}
```

Notes: any Host header (or none path-wise) falls through the gateway to the
API — no DNS entries needed. The record carried genuine hardware: 12 CPU,
7 GB RAM, empty GPU string — independently consistent with the binary's
Guest verdict on that machine (7 GB < 16 GB bar).

### 2. Operator list (master)

`GET /api/nodes` (JWT) showed two records:

- `06f5a0d9 hari linux 12cpu 7gb gpu='' pending` (the guest)
- `2793a018 precheck … denied` (earlier loopback precheck, already cleaned)

### 3. Approve (master)

```bash
curl -s -X POST http://192.168.1.37:80/api/nodes/06f5a0d9/approve \
  -H "Host: control.ros-platform.local" \
  -H "Authorization: Bearer $OP"
```

Response:

```json
{"id":"06f5a0d9","status":"approved"}
```

### 4. First poll (guest) — mint happens here

Guest poll returned HTTP 503 `{"error":"join URL not configured"}`.
Server-side, the same request created exactly one Secret:

- Name: `bootstrap-token-6osx0i`, namespace `kube-system`
- `type: bootstrap.kubernetes.io/token`
- `description: ldndrc-06f5a0d9`
- `auth-extra-groups: system:bootstrappers`
- `expiration`: now + 15 min (verified field present)

Record state after poll: `approved`, `minted: True`. The 503 is correct
behavior with `K3S_JOIN_URL` unset — the token was minted and discarded
instead of handed out with nowhere to join.

### 5. Second poll (guest) — single-mint proof

```json
{"status":"approved"}
```

No token fields. Exactly one bootstrap Secret existed cluster-wide at that
point — the minter ran once and only once across two polls.

### 6. Cleanup (master)

```bash
kubectl -n kube-system delete secret bootstrap-token-6osx0i
```

Result: `0 bootstrap tokens left`. The node record remains `approved`
(no node-delete endpoint exists — acceptable; its token is destroyed and
any future poll re-mints only after re-approval expiry rules).

## What this proves (and what it doesn't)

Proves: cross-machine register → approve → mint → single-mint → cleanup,
with real Wi-Fi, real Traefik ingress, and genuine below-bar hardware on
the guest side. The state machine, the K3s bootstrap-token contract
(type/expiry/groups/description), and the token-once guarantee all held
outside loopback.

Does not prove: full agent join (no `k3s agent` ran — needs a real K3s
master; k3d cannot take real agents), mDNS discovery (P2, `MASTER_IP` was
typed), the TUI Join flow driving this handshake (wired, but the rehearsal
used curl for determinism), deny-after-approve live (covered by unit suite;
live record left approved with token destroyed).

## Follow-ups

- Set `K3S_JOIN_URL` + real K3s master → full agent-install acceptance.
- P2 LAN-mDNS proof from the guest (no more typed IPs).
- Node-record deletion endpoint (currently deny-only terminal states linger).
- Commit this report.
