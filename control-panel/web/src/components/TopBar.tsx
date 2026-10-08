import { Link, NavLink } from "react-router-dom";
import { useAuth } from "../auth";
import { useElapsed } from "../hooks";
import type { Session } from "../api";
import type { View } from "../hooks";

export function TopBar({
  session,
  view,
  onStop,
}: {
  session: Session | null;
  view: View;
  onStop: () => void;
}) {
  const { username, logout } = useAuth();
  const elapsed = useElapsed(session?.createdAt);
  const ready = session?.status === "ready";
  return (
    <header className="topbar">
      <div className="topbar__left">
        <span className="brand" aria-label="LDNDRC home">
          <span className="brand__mark" aria-hidden="true">◆</span> LDNDRC
        </span>
        <nav className="views" aria-label="Workspace view">
          <NavLink
            className={({ isActive }) =>
              `views__btn${isActive && view === "code" ? " views__btn--active" : ""}`
            }
            to="/app?view=code"
          >
            Code
          </NavLink>
          <NavLink
            className={({ isActive }) =>
              `views__btn${isActive && view === "sim" ? " views__btn--active" : ""}`
            }
            to="/app?view=sim"
          >
            Sim
          </NavLink>
          <NavLink
            className={({ isActive }) =>
              `views__btn${isActive && view === "split" ? " views__btn--active" : ""}`
            }
            to="/app?view=split"
          >
            Split
          </NavLink>
        </nav>
      </div>
      <div className="topbar__right">
        <span className={`status ${ready ? "status--ok" : "status--busy"}`}>
          <span
            className="status__dot"
            aria-hidden="true"
          />
          {session ? (ready ? "Ready" : session.status) : "No session"}
          {session && (
            <span className="status__timer mono">{elapsed}</span>
          )}
        </span>
        <a
          className="btn btn--ghost"
          href="/desktop"
          target="_blank"
          rel="noreferrer"
          title="Open full desktop stream in a new tab"
        >
          Desktop ⧉
        </a>
        {session && session.status !== "stopping" && (
          <button type="button" className="btn btn--danger-ghost" onClick={onStop}>
            ⏻ Stop
          </button>
        )}
        <span className="user" title={username}>
          {username}
          <Link className="btn btn--ghost" to="/admin" title="Instructor console">
            Admin
          </Link>
          <button
            type="button"
            className="btn btn--ghost"
            onClick={logout}
            aria-label="Sign out"
          >
            Sign out
          </button>
        </span>
      </div>
    </header>
  );
}
