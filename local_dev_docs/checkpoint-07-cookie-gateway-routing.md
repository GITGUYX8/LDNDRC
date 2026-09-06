# Checkpoint 7 — The Cookie-Powered Workspace Gateway

## The big picture first

The reference project showed us the clean way to connect browser tools to a
student's private workspace: give each tool its own hostname, then let one
gateway decide where the traffic belongs. This avoids asking browser iframes to
carry custom authorization headers and avoids awkward WebSocket path rewriting.

This checkpoint added that browser-facing gateway behavior to the Go control
panel. A logged-in user's cookie now identifies the user, and the gateway can
resolve that user's ready session to its private Kubernetes Service.

## What got built

| File | Friendly name | What it does |
|---|---|---|
| `control-panel/internal/auth/auth.go` | The Browser Pass | Creates, clears, and verifies the HttpOnly session cookie. |
| `control-panel/internal/gateway/session_gateway.go` | The Traffic Director | Routes editor, desktop, and Gazebo hostnames to the user's session Service. |
| `control-panel/internal/gateway/session_gateway_test.go` | The Gateway Exam | Checks that unauthenticated and unready workspace requests are rejected. |
| `control-panel/internal/sessions/store.go` | The Readiness Clerk | Checks Kubernetes-backed readiness before allowing a workspace to receive traffic. |
| `control-panel/internal/sessions/kubernetes.go` | The Status Window | Reads Deployment availability from K3s. |
| `control-panel/internal/httpapi/router.go` | The Login Desk | Sets the browser cookie when login succeeds. |
| `control-panel/cmd/server/main.go` | The Wiring Board | Places the gateway in front of the control-panel API. |
| `manifests/control-panel-ingress.yaml` | The New Front Door | Sends control, editor, desktop, and Gazebo hostnames to the control panel. |
| `manifests/control-panel-deployment.yaml` | The Shared Cookie Rule | Configures the cookie domain for all tool subdomains. |
| `control-panel/README.md` | The Route Map | Documents the reference-aligned hostname setup. |

## How each piece works

### The Browser Pass — `control-panel/internal/auth/auth.go`

Think of this as a wristband that the browser receives after login. It stores
the signed JWT in an HttpOnly cookie, so browser tools can send it automatically
without JavaScript reading the token.

The cookie uses the root path, SameSite Lax, a configurable Secure flag, and an
optional domain. The deployment sets the domain to `.ros-platform.local`, which
lets one login work on `control`, `editor`, `desktop`, and `gazebo` subdomains.

### The Traffic Director — `control-panel/internal/gateway/session_gateway.go`

This is the receptionist who reads the wristband and sends each visitor to the
right room. It recognizes `editor`, `desktop`, and `gazebo` hostnames, checks the
cookie, finds the authenticated user's current session, and chooses the correct
port on that session's Kubernetes Service.

It uses Go's reverse proxy, which handles ordinary HTTP and WebSocket upgrades.
No user-provided upstream URL is accepted, so a browser cannot redirect the
gateway to an arbitrary machine.

### The Gateway Exam — `session_gateway_test.go`

The test sends a request to the editor hostname without a cookie and expects
`401 Unauthorized`. It then gives the request a valid cookie but a session that
is still starting, and expects `503 Service Unavailable` instead of allowing
traffic to a workspace that is not ready.

### The Readiness Clerk — `internal/sessions/store.go`

This clerk does not trust a session label alone. When a Kubernetes provisioner
is available and a session is still provisioning, it asks the provisioner
whether the Deployment has an available replica before the gateway accepts it.

### The Status Window — `internal/sessions/kubernetes.go`

This is the small window into K3s. It reads the session Deployment's
`availableReplicas` value and reports ready only when at least one workspace
replica is available. A missing Deployment is treated as not ready, while real
Kubernetes errors are reported back to the caller.

### The Login Desk — `internal/httpapi/router.go`

After the existing login validation and JWT signing succeed, the API now also
sets the browser session cookie. The JSON token response remains available for
API clients, while browsers can use the cookie naturally with iframes and
WebSockets.

### The Wiring Board — `control-panel/cmd/server/main.go`

The gateway now sits in front of the regular API handler. Tool subdomains are
handled by the gateway; other requests continue to the control-panel routes.
This keeps health checks and session APIs available while adding the new browser
traffic path.

### The New Front Door — `manifests/control-panel-ingress.yaml`

This Ingress is the K3s/Traefik signpost. It sends four hostnames to the
control-panel Service:

```text
control.ros-platform.local
editor.ros-platform.local
desktop.ros-platform.local
gazebo.ros-platform.local
```

The old path-based ingress remains untouched for rollback during migration. It
should be removed only after the new host routes work on a real cluster.

### The Shared Cookie Rule — `control-panel-deployment.yaml`

The deployment now supplies `COOKIE_DOMAIN=.ros-platform.local` and the matching
gateway suffix. Without this setting, a cookie issued on the control hostname
would stay there and never reach the tool subdomains.

## How it all connects

1. The browser logs in through the control hostname.
2. The control panel signs a JWT and sets an HttpOnly cookie.
3. The browser opens the editor, desktop, or Gazebo hostname.
4. The gateway reads the cookie and identifies the user.
5. The session store finds that user's current ready session.
6. The gateway sends traffic to the matching port on that session's private
   Kubernetes Service.
7. HTTP requests and WebSocket connections stay tied to the same workspace.

## Proof it works

- All new and modified YAML manifests passed Ruby YAML parsing.
- `git diff --check` passed.
- The gateway tests were added for missing authentication and unready sessions.
- The existing hardware test was attempted but hung in its simulated Host path
  because the legacy shell script's curl command has no timeout.
- Go tests and formatting were not run because `go` and `gofmt` are unavailable.
- Docker and live K3s verification were not run because Docker daemon access was
  denied and `kubectl` is unavailable.

## The one-liner version

| Piece | One-liner |
|---|---|
| `auth.go` | Gives browsers a secure session wristband they can send automatically. |
| `session_gateway.go` | Sends each tool hostname to the logged-in user's private workspace. |
| `kubernetes.go` | Checks K3s before declaring a workspace ready. |
| `control-panel-ingress.yaml` | Points the four reference-style hostnames at the control panel. |
| `main.go` | Places the gateway in front of the API. |

## What's next

- [ ] Install Go and run `gofmt`, `go mod tidy`, `go test ./...`, and `go build`.
- [ ] Install kubectl or connect to the K3s host and apply the deployment bundle.
- [ ] Configure wildcard DNS or `sslip.io` names for the four hostnames.
- [ ] Build and import the control-panel and workspace images into K3s.
- [ ] Verify HTTP and WebSocket routing with two separate user sessions.
- [ ] Remove the old shared path-based ingress after successful verification.
- [ ] Add CSRF protection to state-changing browser API requests, matching the
      reference project's final security model.
