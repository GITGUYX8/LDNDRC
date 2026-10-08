// Host-gateway helpers. The Go binary serves the SPA on the control host
// (control.ros-platform.local) and proxies tool traffic by subdomain:
// editor (code-server :7682), gazebo (sim :9002), desktop (selkies :8080).
// Path routing (/code /sim) is legacy — iframes must use host routing.

const TOOL_SUBDOMAIN: Record<string, string> = {
  code: "editor",
  sim: "gazebo",
  desktop: "desktop",
};

/** Base domain derived from the current host, e.g. control.ros-platform.local -> ros-platform.local */
export function gatewayBaseDomain(hostname?: string): string {
  const host = (hostname ?? window.location.hostname).toLowerCase();
  const parts = host.split(".");
  // Known tool/control hosts: strip the first label.
  if (
    parts.length >= 3 &&
    ["control", "editor", "desktop", "gazebo"].includes(parts[0])
  ) {
    return parts.slice(1).join(".");
  }
  if (host.endsWith("ros-platform.local")) return "ros-platform.local";
  return "";
}

/** Absolute URL for a tool iframe, or "" when the suffix is unknown (dev on localhost/IP). */
export function toolUrl(view: "code" | "sim" | "desktop"): string {
  const base = gatewayBaseDomain();
  if (!base) return "";
  const proto = window.location.protocol === "https:" ? "https:" : "http:";
  return `${proto}//${TOOL_SUBDOMAIN[view]}.${base}/`;
}
