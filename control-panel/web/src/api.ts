// API wrapper for the Go control panel. Auth model (matches R3):
// - POST /api/auth/login -> {token, csrf_token} + HttpOnly session cookie
// - Bearer token on every /api/sessions call (browsers never attach it
//   cross-site, so classic CSRF is neutered by architecture)
// - X-CSRF-Token on cookie-authed mutations (defense-in-depth + R4 SPA
//   precondition). We send both: cookie via credentials:include, CSRF via
//   header, Bearer via Authorization.

export type SessionStatus =
  | "provisioning"
  | "ready"
  | "stopping"
  | "stopped"
  | "error";

export interface Session {
  id: string;
  username: string;
  workloadName: string;
  serviceName: string;
  rosDomainId: number;
  status: SessionStatus;
  nodeName?: string;
  error?: string;
  createdAt: string;
  updatedAt: string;
}

export const CSRF_HEADER = "X-CSRF-Token";

function creds(): { token: string; csrf: string } {
  return {
    token: sessionStorage.getItem("ldndrc.token") ?? "",
    csrf: sessionStorage.getItem("ldndrc.csrf") ?? "",
  };
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const { token, csrf } = creds();
  const headers = new Headers(init.headers);
  headers.set("Content-Type", "application/json");
  if (token) headers.set("Authorization", `Bearer ${token}`);
  // Mutations carry the per-login CSRF token when we have one. The Go
  // guard only enforces it when the session cookie is present; sending it
  // unconditionally keeps Bearer and cookie paths identical.
  if (csrf && init.method && init.method !== "GET" && init.method !== "HEAD") {
    headers.set(CSRF_HEADER, csrf);
  }
  const res = await fetch(path, { ...init, headers, credentials: "include" });
  let body: unknown = null;
  try {
    body = await res.json();
  } catch {
    body = null;
  }
  if (!res.ok) {
    const msg =
      body && typeof body === "object" && "error" in body
        ? String((body as { error: unknown }).error)
        : `request failed (${res.status})`;
    throw new Error(msg);
  }
  return body as T;
}

export async function login(
  username: string,
  password: string,
): Promise<{ token: string; csrf: string }> {
  const res = await fetch("/api/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify({ username, password }),
  });
  let body: { token?: string; csrf_token?: string; error?: string } = {};
  try {
    body = await res.json();
  } catch {
    body = {};
  }
  if (!res.ok) throw new Error(body.error ?? `login failed (${res.status})`);
  if (!body.token) throw new Error("login failed: no token");
  return { token: body.token, csrf: body.csrf_token ?? "" };
}

export function listSessions(): Promise<{ sessions: Session[] }> {
  return request("/api/sessions");
}

export function createSession(): Promise<{ session: Session }> {
  return request("/api/sessions", { method: "POST", body: "{}" });
}

export function getSession(id: string): Promise<{ session: Session }> {
  return request(`/api/sessions/${encodeURIComponent(id)}`);
}

export function deleteSession(id: string): Promise<{ session: Session }> {
  return request(`/api/sessions/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

// UI-3 — instructor console. Backend seams (all Bearer-authed, CSRF on
// mutations via the shared request() wrapper): GET /api/nodes,
// POST /api/nodes/{id}/approve, POST /api/nodes/{id}/deny.

export type NodeStatus =
  | "pending"
  | "approved"
  | "denied"
  | "joined"
  | "expired";

export interface JoinNode {
  id: string;
  hostname: string;
  os: string;
  arch: string;
  cpu: number;
  ram_gb: number;
  gpu: string;
  status: NodeStatus;
  node_name: string;
  reason?: string;
  requested_at: string;
  approved_at?: string;
}

export function listNodes(): Promise<{ nodes: JoinNode[] }> {
  return request("/api/nodes");
}

export function approveNode(id: string): Promise<{ id: string; status: NodeStatus }> {
  return request(`/api/nodes/${encodeURIComponent(id)}/approve`, {
    method: "POST",
    body: "{}",
  });
}

export function denyNode(
  id: string,
  reason: string,
): Promise<{ id: string; status: NodeStatus }> {
  return request(`/api/nodes/${encodeURIComponent(id)}/deny`, {
    method: "POST",
    body: JSON.stringify({ reason }),
  });
}
