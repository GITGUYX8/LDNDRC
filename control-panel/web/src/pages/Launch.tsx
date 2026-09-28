import { useCallback, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import {
  createSession,
  deleteSession,
  getSession,
  listSessions,
  type Session,
} from "../api";
import { Stepper, stepsFor } from "../components/Stepper";
import { useElapsed } from "../hooks";
import { ToastStack, useToasts } from "../components/Toast";

type Phase = "loading" | "none" | "creating" | "live" | "error";

function isActive(s: Session): boolean {
  return s.status !== "stopped" && s.status !== "error";
}

export function LaunchPage() {
  const [phase, setPhase] = useState<Phase>("loading");
  const [session, setSession] = useState<Session | null>(null);
  const [detail, setDetail] = useState("");
  const [confirmStop, setConfirmStop] = useState(false);
  const pollRef = useRef<number>(0);
  const { toasts, push, dismiss } = useToasts();
  const elapsed = useElapsed(session?.createdAt);

  const refresh = useCallback(async () => {
    try {
      const { sessions } = await listSessions();
      const active = sessions.find(isActive) ?? null;
      if (!active) {
        setSession(null);
        setPhase("none");
        return null;
      }
      // The launch stepper polls the detail endpoint, which self-heals
      // provisioning -> ready (R4 backend prerequisite).
      const { session: full } = await getSession(active.id);
      setSession(full);
      setPhase("live");
      return full;
    } catch (err) {
      setDetail(err instanceof Error ? err.message : "Could not load sessions.");
      setPhase("error");
      return null;
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  // Poll while provisioning/stopping so the stepper terminates.
  useEffect(() => {
    if (
      !session ||
      (session.status !== "provisioning" && session.status !== "stopping")
    ) {
      return;
    }
    pollRef.current = window.setInterval(async () => {
      try {
        const { session: full } = await getSession(session.id);
        setSession(full);
      } catch (err) {
        push(err instanceof Error ? err.message : "Poll failed.");
      }
    }, 2000);
    return () => window.clearInterval(pollRef.current);
  }, [session?.id, session?.status, push]); // eslint-disable-line react-hooks/exhaustive-deps

  async function onLaunch() {
    setPhase("creating");
    setDetail("");
    try {
      const { session: s } = await createSession();
      setSession(s);
      setPhase("live");
      await refresh();
    } catch (err) {
      setDetail(err instanceof Error ? err.message : "Launch failed.");
      setPhase("error");
    }
  }

  async function onStop() {
    if (!session) return;
    try {
      await deleteSession(session.id);
      setConfirmStop(false);
      push("Workspace stopping.");
      await refresh();
    } catch (err) {
      push(err instanceof Error ? err.message : "Stop failed.");
    }
  }

  if (phase === "loading" || phase === "creating") {
    return (
      <main className="launch">
        <div className="panel">
          <h1>{phase === "creating" ? "Creating your session…" : "Loading…"}</h1>
          <Stepper steps={stepsFor("creating")} />
        </div>
      </main>
    );
  }

  if (phase === "error" && !session) {
    return (
      <main className="launch">
        <div className="panel panel--error">
          <h1>Something went wrong</h1>
          <p className="muted">The workspace could not be loaded.</p>
          {detail && (
            <details className="details">
              <summary>Details</summary>
              <pre>{detail}</pre>
            </details>
          )}
          <div className="row">
            <button type="button" className="btn btn--primary" onClick={() => void refresh()}>
              Retry
            </button>
          </div>
        </div>
        <ToastStack toasts={toasts} onDismiss={dismiss} />
      </main>
    );
  }

  if (!session || session.status === "stopped") {
    return (
      <main className="launch">
        <div className="panel panel--hero">
          <h1>Your private ROS 2 workspace</h1>
          <p className="muted">
            We start a private ROS 2 Jazzy + Gazebo Harmonic environment just
            for you.
          </p>
          <button type="button" className="btn btn--primary btn--lg" onClick={() => void onLaunch()}>
            Launch workspace
          </button>
          {session?.status === "stopped" && (
            <p className="muted small">Previous session stopped. Restart whenever ready.</p>
          )}
        </div>
        <ToastStack toasts={toasts} onDismiss={dismiss} />
      </main>
    );
  }

  if (session.status === "provisioning" || session.status === "stopping") {
    return (
      <main className="launch">
        <div className="panel">
          <h1>
            {session.status === "provisioning"
              ? "Starting your workspace…"
              : "Stopping your workspace…"}
          </h1>
          <p className="muted mono" aria-label="Elapsed time">
            Elapsed {elapsed}
          </p>
          <Stepper steps={stepsFor(session.status)} />
          <div className="row">
            <button
              type="button"
              className="btn btn--ghost"
              onClick={() => void onStop()}
            >
              Cancel
            </button>
          </div>
        </div>
        <ToastStack toasts={toasts} onDismiss={dismiss} />
      </main>
    );
  }

  if (session.status === "error") {
    return (
      <main className="launch">
        <div className="panel panel--error">
          <h1>Workspace failed to start</h1>
          <p className="muted">Retry, or ask your instructor if it persists.</p>
          {session.error && (
            <details className="details">
              <summary>Details</summary>
              <pre>{session.error}</pre>
            </details>
          )}
          <div className="row">
            <button type="button" className="btn btn--primary" onClick={() => void onLaunch()}>
              Retry
            </button>
          </div>
        </div>
        <ToastStack toasts={toasts} onDismiss={dismiss} />
      </main>
    );
  }

  // READY
  return (
    <main className="launch">
      <div className="panel">
        <p className="status status--ok">
          <span className="status__dot" aria-hidden="true" /> Ready
          <span className="status__timer mono">{elapsed}</span>
        </p>
        <h1>Workspace ready</h1>
        <div className="cards">
          <div className="card">
            <h2>Code</h2>
            <p className="muted">VS Code in the browser with a built-in terminal.</p>
            <Link className="btn btn--primary" to="/app?view=code">
              Open code
            </Link>
          </div>
          <div className="card">
            <h2>Sim</h2>
            <p className="muted">3D Gazebo view of your robot.</p>
            <Link className="btn btn--primary" to="/app?view=sim">
              Open sim
            </Link>
          </div>
          <div className="card">
            <h2>Desktop</h2>
            <p className="muted">Full Linux desktop for GUI tools like RViz.</p>
            <a className="btn" href="/desktop" target="_blank" rel="noreferrer">
              Open desktop ⧉
            </a>
          </div>
        </div>
        <div className="row">
          <Link className="btn btn--primary btn--lg" to="/app">
            Open workspace
          </Link>
          {!confirmStop ? (
            <button
              type="button"
              className="btn btn--danger-ghost"
              onClick={() => setConfirmStop(true)}
            >
              Stop
            </button>
          ) : (
            <span className="confirm">
              Stop this workspace?
              <button type="button" className="btn btn--danger" onClick={() => void onStop()}>
                Yes, stop
              </button>
              <button
                type="button"
                className="btn btn--ghost"
                onClick={() => setConfirmStop(false)}
              >
                Keep
              </button>
            </span>
          )}
        </div>
      </div>
      <ToastStack toasts={toasts} onDismiss={dismiss} />
    </main>
  );
}
