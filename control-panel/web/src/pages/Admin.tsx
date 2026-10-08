import { useCallback, useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import {
  approveNode,
  deleteSession,
  denyNode,
  listNodes,
  listSessions,
  type JoinNode,
  type NodeStatus,
  type Session,
} from "../api";
import { useElapsed } from "../hooks";
import { ToastStack, useToasts } from "../components/Toast";

function nodeStatusClass(s: NodeStatus): string {
  switch (s) {
    case "joined":
      return "status--ok";
    case "pending":
    case "approved":
      return "status--busy";
    case "denied":
    case "expired":
      return "status--bad";
  }
}

function sessionStatusClass(s: Session["status"]): string {
  switch (s) {
    case "ready":
      return "status--ok";
    case "provisioning":
    case "stopping":
      return "status--busy";
    case "error":
    case "stopped":
      return "status--bad";
  }
}

function countBy<T extends string>(items: T[]): Record<string, number> {
  const out: Record<string, number> = {};
  for (const item of items) out[item] = (out[item] ?? 0) + 1;
  return out;
}

type SessionFilter = "all" | Session["status"];

const FILTERS: SessionFilter[] = [
  "all",
  "provisioning",
  "ready",
  "stopping",
  "stopped",
  "error",
];

function SessionCard({
  session,
  onStop,
  stopping,
}: {
  session: Session;
  onStop: (id: string) => void;
  stopping: boolean;
}) {
  const age = useElapsed(session.createdAt);
  const active = session.status === "ready" || session.status === "provisioning";
  return (
    <article className="card">
      <p className={`status ${sessionStatusClass(session.status)}`}>
        <span className="status__dot" aria-hidden="true" /> {session.status}
        <span className="status__timer mono">{age}</span>
      </p>
      <h2 className="mono small">{session.id}</h2>
      <dl className="facts">
        <div>
          <dt>User</dt>
          <dd>{session.username}</dd>
        </div>
        <div>
          <dt>Node</dt>
          <dd className="mono">{session.nodeName || "—"}</dd>
        </div>
        <div>
          <dt>ROS domain</dt>
          <dd className="mono">{session.rosDomainId}</dd>
        </div>
        <div>
          <dt>Created</dt>
          <dd className="mono">{new Date(session.createdAt).toLocaleString()}</dd>
        </div>
      </dl>
      {session.error && (
        <details className="details">
          <summary>Error detail</summary>
          <pre>{session.error}</pre>
        </details>
      )}
      {active && (
        <div className="row">
          <button
            type="button"
            className="btn btn--danger-ghost"
            disabled={stopping}
            onClick={() => onStop(session.id)}
          >
            Stop
          </button>
        </div>
      )}
    </article>
  );
}

function NodeCard({
  node,
  busy,
  denying,
  reason,
  onApprove,
  onDenyStart,
  onDenyCancel,
  onReason,
  onDeny,
}: {
  node: JoinNode;
  busy: boolean;
  denying: boolean;
  reason: string;
  onApprove: () => void;
  onDenyStart: () => void;
  onDenyCancel: () => void;
  onReason: (v: string) => void;
  onDeny: () => void;
}) {
  const actionable = node.status === "pending" || node.status === "expired";
  return (
    <article className="card">
      <p className={`status ${nodeStatusClass(node.status)}`}>
        <span className="status__dot" aria-hidden="true" /> {node.status}
      </p>
      <h2 className="mono small">{node.hostname}</h2>
      <dl className="facts">
        <div>
          <dt>Specs</dt>
          <dd>
            {node.os}/{node.arch} · {node.cpu} CPU · {node.ram_gb}GB
            {node.gpu ? ` · ${node.gpu}` : ""}
          </dd>
        </div>
        <div>
          <dt>Node name</dt>
          <dd className="mono">{node.node_name}</dd>
        </div>
        <div>
          <dt>Requested</dt>
          <dd className="mono">
            {new Date(node.requested_at).toLocaleString()}
          </dd>
        </div>
        {node.reason && (
          <div>
            <dt>Reason</dt>
            <dd>{node.reason}</dd>
          </div>
        )}
      </dl>
      {actionable &&
        (denying ? (
          <div className="stack">
            <input
              className="input"
              type="text"
              placeholder="Reason (optional)"
              value={reason}
              onChange={(e) => onReason(e.target.value)}
              aria-label="Deny reason"
            />
            <span className="row">
              <button
                type="button"
                className="btn btn--danger"
                disabled={busy}
                onClick={onDeny}
              >
                Deny
              </button>
              <button
                type="button"
                className="btn btn--ghost"
                onClick={onDenyCancel}
              >
                Cancel
              </button>
            </span>
          </div>
        ) : (
          <div className="row">
            <button
              type="button"
              className="btn btn--primary"
              disabled={busy}
              onClick={onApprove}
            >
              Approve
            </button>
            {node.status === "pending" && (
              <button
                type="button"
                className="btn btn--danger-ghost"
                disabled={busy}
                onClick={onDenyStart}
              >
                Deny…
              </button>
            )}
          </div>
        ))}
    </article>
  );
}

export function AdminPage() {
  const [nodes, setNodes] = useState<JoinNode[]>([]);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busyNode, setBusyNode] = useState("");
  const [stoppingSession, setStoppingSession] = useState("");
  const [denyFor, setDenyFor] = useState("");
  const [denyReason, setDenyReason] = useState("");
  const [filter, setFilter] = useState<SessionFilter>("all");
  const { toasts, push, dismiss } = useToasts();

  const refresh = useCallback(async () => {
    try {
      const [{ nodes: n }, { sessions: s }] = await Promise.all([
        listNodes(),
        listSessions(),
      ]);
      setNodes(n);
      setSessions(s);
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load admin data.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
    const t = window.setInterval(() => void refresh(), 5000);
    return () => window.clearInterval(t);
  }, [refresh]);

  async function onApprove(id: string) {
    setBusyNode(id);
    try {
      await approveNode(id);
      push("Node approved — the laptop can now poll for its join token.");
      await refresh();
    } catch (err) {
      push(err instanceof Error ? err.message : "Approve failed.");
    } finally {
      setBusyNode("");
    }
  }

  async function onDeny(id: string) {
    setBusyNode(id);
    try {
      await denyNode(id, denyReason.trim());
      push("Node denied.");
      setDenyFor("");
      setDenyReason("");
      await refresh();
    } catch (err) {
      push(err instanceof Error ? err.message : "Deny failed.");
    } finally {
      setBusyNode("");
    }
  }

  async function onStopSession(id: string) {
    setStoppingSession(id);
    try {
      await deleteSession(id);
      push("Session stopping.");
      await refresh();
    } catch (err) {
      push(err instanceof Error ? err.message : "Stop failed.");
    } finally {
      setStoppingSession("");
    }
  }

  const nodeCounts = countBy(nodes.map((n) => n.status));
  const sessionCounts = countBy(sessions.map((s) => s.status));

  // User cards aggregate sessions per account (reference: user cards).
  // Backend note: GET /api/sessions is scoped to the caller, so today
  // this is one card per signed-in instructor; a list-all seam would
  // populate the whole room here without changing this view.
  const users = useMemo(() => {
    const byUser = new Map<string, Session[]>();
    for (const s of sessions) {
      const list = byUser.get(s.username) ?? [];
      list.push(s);
      byUser.set(s.username, list);
    }
    return [...byUser.entries()];
  }, [sessions]);

  const visibleSessions =
    filter === "all" ? sessions : sessions.filter((s) => s.status === filter);

  return (
    <main className="admin">
      <div className="panel panel--wide">
        <p className="muted small">
          <Link to="/launch">← Back to launch</Link>
        </p>
        <div className="admin__head">
          <h1>Instructor console</h1>
          <button
            type="button"
            className="btn btn--ghost"
            onClick={() => void refresh()}
            disabled={loading}
          >
            ↻ Refresh
          </button>
        </div>
        {error && (
          <p className="banner banner--error" role="alert">
            {error}
          </p>
        )}

        <section aria-label="Fleet metrics">
          <h2>Fleet</h2>
          <ul className="metrics">
            <li className="metric">
              <span className="metric__value mono">{nodes.length}</span>
              <span className="metric__label">hosts</span>
            </li>
            {(["pending", "approved", "joined", "denied", "expired"] as NodeStatus[]).map(
              (s) => (
                <li className="metric" key={s}>
                  <span className="metric__value mono">{nodeCounts[s] ?? 0}</span>
                  <span className={`status ${nodeStatusClass(s)}`}>
                    <span className="status__dot" aria-hidden="true" /> {s}
                  </span>
                </li>
              ),
            )}
            <li className="metric">
              <span className="metric__value mono">{sessions.length}</span>
              <span className="metric__label">sessions</span>
            </li>
            {Object.entries(sessionCounts).map(([s, n]) => (
              <li className="metric" key={s}>
                <span className="metric__value mono">{n}</span>
                <span
                  className={`status ${sessionStatusClass(s as Session["status"])}`}
                >
                  <span className="status__dot" aria-hidden="true" /> {s}
                </span>
              </li>
            ))}
          </ul>
        </section>

        <section aria-label="Users">
          <h2>Users</h2>
          {loading ? (
            <p className="muted">Loading…</p>
          ) : users.length === 0 ? (
            <p className="muted">No sessions yet — user cards appear here.</p>
          ) : (
            <div className="cards">
              {users.map(([username, list]) => {
                const counts = countBy(list.map((s) => s.status));
                return (
                  <article className="card" key={username}>
                    <p className="usercard__avatar" aria-hidden="true">
                      {username.slice(0, 1).toUpperCase()}
                    </p>
                    <h2>{username}</h2>
                    <p className="muted small">
                      {list.length} session{list.length === 1 ? "" : "s"}
                      {counts["ready"]
                        ? ` · ${counts["ready"]} ready`
                        : ""}
                      {counts["provisioning"]
                        ? ` · ${counts["provisioning"]} provisioning`
                        : ""}
                    </p>
                  </article>
                );
              })}
            </div>
          )}
        </section>

        <section aria-label="Sessions">
          <div className="admin__head">
            <h2>Sessions</h2>
            <div
              className="chips"
              role="group"
              aria-label="Filter sessions by status"
            >
              {FILTERS.map((f) => (
                <button
                  key={f}
                  type="button"
                  className={`chip${filter === f ? " chip--active" : ""}`}
                  aria-pressed={filter === f}
                  onClick={() => setFilter(f)}
                >
                  {f}
                  {f !== "all" && sessionCounts[f] ? ` (${sessionCounts[f]})` : ""}
                </button>
              ))}
            </div>
          </div>
          {loading ? (
            <p className="muted">Loading…</p>
          ) : visibleSessions.length === 0 ? (
            <p className="muted">
              {filter === "all"
                ? "No sessions for your account."
                : `No ${filter} sessions.`}
            </p>
          ) : (
            <div className="cards cards--admin">
              {visibleSessions.map((s) => (
                <SessionCard
                  key={s.id}
                  session={s}
                  stopping={stoppingSession === s.id}
                  onStop={(id) => void onStopSession(id)}
                />
              ))}
            </div>
          )}
        </section>

        <section aria-label="Host onboarding requests">
          <h2>Hosts</h2>
          {loading ? (
            <p className="muted">Loading…</p>
          ) : nodes.length === 0 ? (
            <p className="muted">
              No join requests yet. Laptops register from the join TUI; they
              appear here as <span className="mono">pending</span>.
            </p>
          ) : (
            <div className="cards cards--admin">
              {nodes.map((n) => (
                <NodeCard
                  key={n.id}
                  node={n}
                  busy={busyNode === n.id}
                  denying={denyFor === n.id}
                  reason={denyReason}
                  onApprove={() => void onApprove(n.id)}
                  onDenyStart={() => setDenyFor(n.id)}
                  onDenyCancel={() => {
                    setDenyFor("");
                    setDenyReason("");
                  }}
                  onReason={setDenyReason}
                  onDeny={() => void onDeny(n.id)}
                />
              ))}
            </div>
          )}
        </section>
      </div>
      <ToastStack toasts={toasts} onDismiss={dismiss} />
    </main>
  );
}
