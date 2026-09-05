"""Demo stand-in workspace server — plumbing test only, not a real workspace.

Listens on the session editor port (7682) so the readiness probe passes and
the gateway editor route returns 200. Every response identifies itself as a
stand-in so a green dashboard is never mistaken for a working ROS workspace.
"""

from http.server import BaseHTTPRequestHandler, HTTPServer

PORT = 7682
BODY = (
    "ldndrc-demo-standin: plumbing only, not a ROS workspace.\n"
    "If you see this through the gateway, Traefik -> cookie auth -> "
    "session Service routing works end to end.\n"
).encode()


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):  # noqa: N802 - stdlib naming
        self.send_response(200)
        self.send_header("Content-Type", "text/plain")
        self.send_header("Content-Length", str(len(BODY)))
        self.end_headers()
        self.wfile.write(BODY)

    def log_message(self, *args):
        pass


if __name__ == "__main__":
    HTTPServer(("0.0.0.0", PORT), Handler).serve_forever()
