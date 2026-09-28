import { Link } from "react-router-dom";
import type { View } from "../hooks";

/** Icon-only rail duplicating the view switcher for muscle memory. */
export function Rail({
  view,
  dropped,
  onReconnect,
}: {
  view: View;
  dropped: boolean;
  onReconnect: () => void;
}) {
  return (
    <nav className="rail" aria-label="Workspace tools">
      <Link
        to="/app?view=code"
        className={`rail__btn${view === "code" ? " rail__btn--active" : ""}`}
        title="Code"
        aria-label="Code"
        aria-current={view === "code" ? "page" : undefined}
      >
        <span aria-hidden="true">{"</>"}</span>
      </Link>
      <Link
        to="/app?view=sim"
        className={`rail__btn${view === "sim" ? " rail__btn--active" : ""}`}
        title="Sim"
        aria-label="Sim"
        aria-current={view === "sim" ? "page" : undefined}
      >
        <span aria-hidden="true">◉</span>
      </Link>
      <Link
        to="/app?view=split"
        className={`rail__btn${view === "split" ? " rail__btn--active" : ""}`}
        title="Split: code and sim side by side"
        aria-label="Split"
        aria-current={view === "split" ? "page" : undefined}
      >
        <span aria-hidden="true">▤</span>
      </Link>
      <a
        className="rail__btn"
        href="/desktop"
        target="_blank"
        rel="noreferrer"
        title="Desktop (new tab)"
        aria-label="Desktop, opens in new tab"
      >
        <span aria-hidden="true">▦</span>
      </a>
      <button
        type="button"
        className={`rail__btn${dropped ? " rail__btn--alert" : ""}`}
        title="Reconnect streams"
        aria-label="Reconnect streams"
        onClick={onReconnect}
      >
        <span aria-hidden="true">↻</span>
        {dropped && (
          <span className="rail__badge" aria-hidden="true">!</span>
        )}
      </button>
    </nav>
  );
}
