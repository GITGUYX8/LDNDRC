#!/usr/bin/env python3
"""LDNDRC captive-portal trap (onboarding AP harness).

nftables REDIRECTs every port-80 packet arriving on the onboarding
hotspot to this server — including the OS connectivity probes
(gstatic / msftconnecttest / connectivity-check.ubuntu.com), which is
what makes the OS pop its "sign in to this network" prompt. Every
request is answered with a 302 to the real portal page.

The portal URL is either given explicitly with --panel-url, or derived
per request from the hotspot interface's own IPv4 address (so it keeps
working when NetworkManager picks a different subnet).

This is a demo/workshop harness, not gateway enforcement: it intercepts
traffic, it does not authorize clients. See docs/lan-onboarding-portal.md.

Run standalone (no root needed):
    python3 captive-trap.py --iface wlp0s20f3 --panel-port 8082
    python3 captive-trap.py --panel-url http://192.168.4.1:8082/portal

Normally started for you by ./run.sh start, which also renders the nft
and dnsmasq configs and prints the privileged setup block.
"""
import argparse
import subprocess
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

# Address ranges an onboarding hotspot is likely to use, most specific
# first. Any of these beats the master's home-LAN address, which clients
# on the hotspot cannot reach.
HOTSPOT_PREFIXES = ("192.168.4.", "10.42.")

DEFAULT_IFACE_FALLBACK_IP = "192.168.4.1"


def iface_ipv4(iface: str):
    """Return the (preferred) IPv4 address of `iface`, or None."""
    try:
        out = subprocess.run(
            ["ip", "-4", "-o", "addr", "show", "dev", iface],
            capture_output=True, text=True, timeout=2, check=True,
        ).stdout
    except Exception:
        return None

    fallback = None
    for line in out.splitlines():
        parts = line.split()
        if len(parts) >= 4 and parts[2] == "inet":
            ip = parts[3].split("/")[0]
            if ip.startswith("127."):
                continue
            if ip.startswith(HOTSPOT_PREFIXES):
                return ip
            fallback = fallback or ip
    return fallback


def resolve_portal_factory(args):
    """Return a zero-arg callable giving the current portal URL."""
    if args.panel_url:
        fixed = args.panel_url.rstrip("/")
        return lambda: fixed

    def dynamic() -> str:
        ip = iface_ipv4(args.iface) or DEFAULT_IFACE_FALLBACK_IP
        return f"http://{ip}:{args.panel_port}{args.portal_path}"

    return dynamic


def resolve_portal(args) -> str:
    """Exact portal URL to redirect clients to, right now."""
    return resolve_portal_factory(args)()


def make_handler(portal_url):
    class Trap(BaseHTTPRequestHandler):
        protocol_version = "HTTP/1.1"

        def _fallback_page(self) -> bytes:
            url = portal_url()
            return (
                "<!doctype html><html><head><meta charset='utf-8'>"
                f"<meta http-equiv='refresh' content='0;url={url}'>"
                "<title>LDNDRC onboarding</title></head><body>"
                "<p>Redirecting to the LDNDRC onboarding portal… "
                f"<a href='{url}'>open it</a> if nothing happens.</p>"
                "</body></html>"
            ).encode()

        def _redirect(self, include_body: bool) -> None:
            url = portal_url()
            self.send_response(302)
            self.send_header("Location", url)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Cache-Control", "no-store")
            if include_body:
                body = self._fallback_page()
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)
            else:
                self.send_header("Content-Length", "0")
                self.end_headers()

        def do_GET(self) -> None:
            self._redirect(include_body=True)

        def do_HEAD(self) -> None:
            self._redirect(include_body=False)

        def do_POST(self) -> None:  # some apps POST through portals
            self._redirect(include_body=False)

        def log_message(self, fmt: str, *args) -> None:
            # Keep a visible trace of what the hotspot intercepted.
            print(f"[trap] {self.client_address[0]} {fmt % args}", flush=True)

    return Trap


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Redirect intercepted port-80 traffic to the LDNDRC portal.",
    )
    parser.add_argument(
        "--iface",
        help="onboarding hotspot interface whose IPv4 to advertise "
             "(e.g. wlp0s20f3)",
    )
    parser.add_argument(
        "--panel-port", type=int, default=8082,
        help="control-panel port used when building the URL (default 8082)",
    )
    parser.add_argument(
        "--portal-path", default="/portal",
        help="portal path appended to the panel URL (default /portal)",
    )
    parser.add_argument(
        "--panel-url",
        help="exact portal URL; overrides --iface/--panel-port "
             "(e.g. http://192.168.4.1:8082/portal)",
    )
    parser.add_argument(
        "--listen-port", type=int, default=8090,
        help="port the trap listens on before nft redirect (default 8090)",
    )
    parser.add_argument(
        "--bind", default="0.0.0.0",
        help="bind address (default 0.0.0.0 — nft rewrites the "
             "destination to the hotspot iface address)",
    )
    parser.add_argument(
        "--once", action="store_true",
        help="print the resolved portal URL and exit (config check)",
    )
    args = parser.parse_args()

    if not args.panel_url and not args.iface:
        parser.error("give --panel-url or --iface (or both)")

    portal_url = resolve_portal(args)
    if args.once:
        print(portal_url)
        return 0

    print(
        f"[trap] listening on {args.bind}:{args.listen_port} — "
        f"redirecting everything to {portal_url}",
        flush=True,
    )
    try:
        ThreadingHTTPServer(
            (args.bind, args.listen_port), make_handler(resolve_portal_factory(args))
        ).serve_forever()
    except KeyboardInterrupt:
        return 0
    return 0


if __name__ == "__main__":
    sys.exit(main())
