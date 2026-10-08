import { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { deleteSession, getSession, listSessions, type Session } from "../api";
import { gatewayBaseDomain, toolUrl } from "../gateway";
import { parseView, type View } from "../hooks";
import { TopBar } from "../components/TopBar";
import { Rail } from "../components/Rail";
import { ToastStack, useToasts } from "../components/Toast";

const RATIO_KEY = "ldndrc.splitRatio";

function loadRatio(): number {
  const raw = localStorage.getItem(RATIO_KEY);
  const v = raw ? Number(raw) : 0.5;
  return Number.isFinite(v) && v > 0.2 && v < 0.8 ? v : 0.5;
}

/**
 * Workspace shell (UI-1 core + UI-2 reconnect/split persistence).
 * Code and Sim iframes stay mounted across view switches (CSS visibility,
 * not unmount) so code-server and the sim socket never reconnect on view
 * change. Sim is an iframe to the gazebo gateway host in UI-1; the mounted
 * gzweb component lands with the real workspace image (R6).
 */
export function WorkspacePage() {
  const [params, setParams] = useSearchParams();
  const view: View = parseView(params.toString());
  const [session, setSession] = useState<Session | null>(null);
  const [ratio, setRatio] = useState(loadRatio);
  const [dragging, setDragging] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [dropped, setDropped] = useState(false);
  const [confirmStop, setConfirmStop] = useState(false);
  const stageRef = useRef<HTMLDivElement>(null);
  const failsRef = useRef(0);
  const navigate = useNavigate();
  const { toasts, push, dismiss } = useToasts();

  const setView = (v: View) => setParams(v === "code" ? {} : { view: v });

  useEffect(() => {
    (async () => {
      try {
        const { sessions } = await listSessions();
        const active =
          sessions.find((s) => s.status !== "stopped" && s.status !== "error") ??
          null;
        if (!active) {
          navigate("/launch", { replace: true });
          return;
        }
        if (active.status !== "ready") {
          // Provisioning sessions belong on the launch stepper.
          navigate("/launch", { replace: true });
          return;
        }
        const { session: full } = await getSession(active.id);
        setSession(full);
      } catch {
        navigate("/launch", { replace: true });
      }
    })();
  }, [navigate]);

  // Ambient status: re-probe every 15s so a stopped/errored session does not
  // strand the user in an empty shell.
  useEffect(() => {
    if (!session) return;
    const t = window.setInterval(async () => {
      try {
        const { session: full } = await getSession(session.id);
        setSession(full);
        if (full.status !== "ready") navigate("/launch", { replace: true });
      } catch {
        /* keep shell on transient poll errors */
      }
    }, 15000);
    return () => window.clearInterval(t);
  }, [session?.id, navigate]); // eslint-disable-line react-hooks/exhaustive-deps

  const onStop = useCallback(() => setConfirmStop(true), []);

  async function confirmStopNow() {
    if (!session) return;
    try {
      await deleteSession(session.id);
      navigate("/launch", { replace: true });
    } catch (err) {
      push(err instanceof Error ? err.message : "Stop failed.");
    }
  }

  function reconnect() {
    failsRef.current = 0;
    setDropped(false);
    setReloadKey((k) => k + 1);
    push("Reconnecting streams…");
  }

  function noteDrop(tool: string) {
    failsRef.current += 1;
    if (failsRef.current >= 1) {
      setDropped(true);
      push(`${tool} stream dropped. Reconnect from the rail.`);
    }
  }

  function onDividerDown(e: React.PointerEvent) {
    e.preventDefault();
    setDragging(true);
    const move = (ev: PointerEvent) => {
      const el = stageRef.current;
      if (!el) return;
      const rect = el.getBoundingClientRect();
      const v = (ev.clientX - rect.left) / rect.width;
      setRatio(Math.min(0.8, Math.max(0.2, v)));
    };
    const up = () => {
      setDragging(false);
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
  }

  useEffect(() => {
    if (!dragging) localStorage.setItem(RATIO_KEY, String(ratio));
  }, [dragging, ratio]);

  const codeSrc = toolUrl("code");
  const simSrc = toolUrl("sim");
  const unknownSuffix = !gatewayBaseDomain();

  return (
    <div className="shell">
      <TopBar session={session} view={view} onStop={onStop} />
      <div className="shell__body">
        <Rail view={view} dropped={dropped} onReconnect={reconnect} />
        <div className="stage" ref={stageRef}>
          {unknownSuffix && (
            <p className="banner banner--warn" role="note">
              Tool iframes need a{" "}
              <code>*.ros-platform.local</code> host. You are on{" "}
              <code>{window.location.hostname}</code> — open this page via{" "}
              <code>control.ros-platform.local</code> for live Code/Sim.
            </p>
          )}
          {view !== "sim" && (
            <section
              className="stage__pane"
              aria-label="Code"
              style={
                view === "split"
                  ? { width: `${Math.round(ratio * 100)}%` }
                  : undefined
              }
              hidden={false}
              data-visible={view === "code" || view === "split"}
            >
              {codeSrc ? (
                <iframe
                  key={`code-${reloadKey}`}
                  title="Code"
                  src={codeSrc}
                  className="stage__frame"
                  allow="clipboard-read; clipboard-write"
                  onLoad={() => setDropped(false)}
                />
              ) : (
                <div className="stage__empty">
                  <p>Code stream unavailable on this host.</p>
                  <button type="button" className="btn" onClick={reconnect}>
                    Reconnect
                  </button>
                </div>
              )}
            </section>
          )}
          {view === "split" && (
            <div
              className="stage__divider"
              role="separator"
              aria-orientation="vertical"
              aria-label="Resize code and sim panes"
              tabIndex={0}
              onPointerDown={onDividerDown}
              onKeyDown={(e) => {
                if (e.key === "ArrowLeft") setRatio((r) => Math.max(0.2, r - 0.05));
                if (e.key === "ArrowRight") setRatio((r) => Math.min(0.8, r + 0.05));
              }}
            />
          )}
          {view !== "code" && (
            <section
              className="stage__pane"
              aria-label="Sim"
              style={
                view === "split"
                  ? { width: `${Math.round((1 - ratio) * 100)}%` }
                  : undefined
              }
              data-visible={view === "sim" || view === "split"}
            >
              {simSrc ? (
                <iframe
                  key={`sim-${reloadKey}`}
                  title="Sim"
                  src={simSrc}
                  className="stage__frame"
                  onLoad={() => setDropped(false)}
                  onError={() => noteDrop("Sim")}
                />
              ) : (
                <div className="stage__empty">
                  <p>Sim stream unavailable on this host.</p>
                  <button type="button" className="btn" onClick={reconnect}>
                    Reconnect
                  </button>
                </div>
              )}
            </section>
          )}
          {/* Keep the hidden pane mounted for instant switching: visibility
              toggling (not unmount) preserves code-server + sim sockets. */}
          {view === "sim" && codeSrc && (
            <iframe
              key={`code-hidden-${reloadKey}`}
              title="Code (background)"
              src={codeSrc}
              className="stage__frame stage__frame--hidden"
              tabIndex={-1}
              aria-hidden="true"
            />
          )}
          {view === "code" && simSrc && (
            <iframe
              key={`sim-hidden-${reloadKey}`}
              title="Sim (background)"
              src={simSrc}
              className="stage__frame stage__frame--hidden"
              tabIndex={-1}
              aria-hidden="true"
            />
          )}
        </div>
      </div>
      {view !== "code" && (
        <div className="sr-only" aria-live="polite">
          {view === "sim" ? "Sim view" : "Split view"}
        </div>
      )}
      {confirmStop && (
        <div className="modal" role="dialog" aria-modal="true" aria-label="Stop workspace">
          <div className="modal__card">
            <h2>Stop this workspace?</h2>
            <p className="muted">
              Unsaved work inside the pod is kept on its home volume, but
              running processes stop.
            </p>
            <div className="row">
              <button type="button" className="btn btn--danger" onClick={() => void confirmStopNow()}>
                Yes, stop
              </button>
              <button
                type="button"
                className="btn btn--ghost"
                onClick={() => setConfirmStop(false)}
              >
                Keep running
              </button>
            </div>
          </div>
        </div>
      )}
      <div className="viewtabs" role="navigation" aria-label="Switch view">
        {(["code", "sim", "split"] as View[]).map((v) => (
          <button
            key={v}
            type="button"
            className={`btn${view === v ? " btn--primary" : ""}`}
            aria-pressed={view === v}
            onClick={() => setView(v)}
          >
            {v === "code" ? "Code" : v === "sim" ? "Sim" : "Split"}
          </button>
        ))}
      </div>
      <ToastStack toasts={toasts} onDismiss={dismiss} />
    </div>
  );
}
