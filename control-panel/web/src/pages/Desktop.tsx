import { useState } from "react";
import { Link } from "react-router-dom";
import { gatewayBaseDomain, toolUrl } from "../gateway";

const HINT_KEY = "ldndrc.desktopHintDismissed";

/**
 * Desktop escape hatch (UI-2): Selkies stream full-bleed in its own tab
 * (it needs keyboard/mouse capture), minimal wrapper with a Back pill.
 */
export function DesktopPage() {
  const [hintDismissed, setHintDismissed] = useState(
    () => localStorage.getItem(HINT_KEY) === "1",
  );
  const src = toolUrl("desktop");
  const unknown = !gatewayBaseDomain();

  function dismiss() {
    localStorage.setItem(HINT_KEY, "1");
    setHintDismissed(true);
  }

  return (
    <main className="desktop">
      <div className="desktop__bar">
        <Link className="btn btn--ghost" to="/app">
          ← Back to workspace
        </Link>
        <span className="muted small">
          Full desktop stream — keyboard capture works best here.
        </span>
      </div>
      {!hintDismissed && (
        <p className="banner banner--warn" role="note">
          Click inside the desktop once to capture your keyboard. Press{" "}
          <kbd>Ctrl</kbd>+<kbd>Alt</kbd> (or <kbd>Esc</kbd>) to release.
          <button type="button" className="btn btn--ghost" onClick={dismiss}>
            Got it
          </button>
        </p>
      )}
      {src && !unknown ? (
        <iframe title="Desktop" src={src} className="desktop__frame" allow="clipboard-read; clipboard-write; fullscreen" />
      ) : (
        <div className="stage__empty">
          <p>
            Desktop stream needs a <code>*.ros-platform.local</code> host. Open
            via <code>control.ros-platform.local/desktop</code>.
          </p>
        </div>
      )}
    </main>
  );
}
