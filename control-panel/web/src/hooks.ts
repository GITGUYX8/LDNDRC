import { useEffect, useState } from "react";

export type View = "code" | "sim" | "split";

export function useElapsed(sinceIso: string | undefined, tickMs = 1000): string {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const t = window.setInterval(() => setNow(Date.now()), tickMs);
    return () => window.clearInterval(t);
  }, [tickMs]);
  if (!sinceIso) return "00:00:00";
  const ms = Math.max(0, now - Date.parse(sinceIso));
  const s = Math.floor(ms / 1000);
  const h = String(Math.floor(s / 3600)).padStart(2, "0");
  const m = String(Math.floor((s % 3600) / 60)).padStart(2, "0");
  const sec = String(s % 60).padStart(2, "0");
  return `${h}:${m}:${sec}`;
}

export function parseView(search: string): View {
  const v = new URLSearchParams(search).get("view");
  return v === "sim" || v === "split" ? v : "code";
}
