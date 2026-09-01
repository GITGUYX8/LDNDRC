# Gazebo WebSocket Viewer Test

This is a small Vite app for testing the modern Gazebo WebSocket rendering path outside the student workspace container.

The intended model is:

1. Gazebo Sim runs inside the workspace container.
2. The student launches a Gazebo world from code-server's terminal or
   `xfce4-terminal` in the desktop.
3. A supervised `gazebo-web` process starts the Gazebo websocket bridge.
4. The WebSocket server exposes live simulation scene/state on a port such as `9002`.
5. This browser app runs outside the container and connects to that WebSocket URL.
6. The browser renders the scene with the `gzweb` npm package.

This is a test harness, not production cockpit UI.

## Files

- `package.json` defines a private Vite app with `gzweb`.
- `index.html` provides the page shell, URL input, connect controls, scene container, and debug panel.
- `src/main.js` dynamically loads `gzweb`, creates a `SceneManager`, connects it to the selected WebSocket URL, and logs detailed connection/rendering diagnostics.
- `src/style.css` lays out the viewer and debug panel.
- `README.md` documents the workflow and cockpit integration notes.

## Install Dependencies

Use Node 24 or newer. The `gzweb` package currently targets modern Node/npm tooling, and older Node versions may fail during dependency installation or Vite startup.

```bash
cd tests/gazebo-websocket-viewer
npm install
```

## Run The Test Frontend

```bash
cd tests/gazebo-websocket-viewer
npm run dev -- --host 127.0.0.1
```

Open the Vite URL printed by the command, usually:

```text
http://127.0.0.1:5173/
```

The WebSocket URL field defaults to:

```text
ws://localhost:9002
```

Auto-connect is enabled by default. You can open the viewer before Gazebo is running; it will retry until the websocket endpoint appears. You can also click **Connect** manually.

## Start Or Verify The Container-Side Gazebo WebSocket Server

The current minimal workspace image has three relevant services:

- `code-server` is supervised and gives the student a browser editor plus
  persistent integrated terminals.
- `selkies` is supervised and exposes the desktop, including `xfce4-terminal`.
- `gazebo-web` is supervised and starts the Gazebo websocket bridge from `/opt/gazebo-web/websocket.gzlaunch`.

Gazebo Sim itself is intentionally not supervised. Students launch it in the foreground so they can choose normal Gazebo worlds and see Gazebo logs directly in their terminal.

Build the images if needed:

```bash
podman build -t localhost/eysip-ros2-workshop:jazzy-gz-base apps/student-workspace/base
podman build -t localhost/eysip-ros2-workshop:minimal apps/student-workspace/minimal
```

Run the minimal workspace and publish the WebSocket port to the host:

```bash
podman run --rm --name ros-minimal -p 9002:9002 -p 7682:7682 -p 8080:8080 localhost/eysip-ros2-workshop:minimal
```

In another terminal, verify supervisor and logs:

```bash
podman exec ros-minimal supervisorctl status
podman exec ros-minimal bash -lc 'tail -n 100 /tmp/gazebo-web.log /tmp/gazebo-web.err.log'
```

Open code-server at:

```text
http://localhost:7682/
```

Then launch a world in a code-server terminal. For TurtleBot3:

```bash
turtlebot3-sim
```

For an arbitrary SDF world, use the normal Gazebo server-only form:

```bash
gz sim -s -v 3 ~/dev_ws/src/my_workshop/worlds/maze.sdf -r
```

Gazebo logs remain in that terminal. Stop the simulation with `Ctrl-C`.

To verify that the websocket server is listening:

```bash
podman exec ros-minimal bash -lc 'ss -ltnp | grep 9002 || netstat -ltnp | grep 9002'
```

You can also inspect Gazebo topics:

```bash
podman exec ros-minimal bash -lc 'gz topic -l'
```

This image currently uses the Harmonic-compatible `gz-launch` websocket plugin:

```xml
<plugin
  name="gz::launch::WebsocketServer"
  filename="gz-launch-websocket-server">
  <port>9002</port>
  <publication_hz>30</publication_hz>
  <max_connections>-1</max_connections>
</plugin>
```

Student worlds still need to publish scene data. For workshop materials, document adding `SceneBroadcaster` to each `.sdf`:

```xml
<plugin
  filename="gz-sim-scene-broadcaster-system"
  name="gz::sim::systems::SceneBroadcaster">
</plugin>
```

Newer Gazebo releases document a `gz-sim-websocket-server-system` that can be added directly to an SDF or injected at runtime, but this container reports Gazebo Sim 8.11.0 and does not have that shared library installed.

## Test With `ws://localhost:9002`

When the container is running with `-p 9002:9002`, the browser app can connect to:

```text
ws://localhost:9002
```

Expected successful behavior:

- The status badge moves from `Idle` to `Waiting` if Gazebo is not running yet.
- After the student launches a world, the status badge moves to `Connecting`.
- The debug panel reports that the raw WebSocket probe opened.
- The debug panel reports that `gzweb` loaded and a `SceneManager` was created.
- A WebGL canvas appears in the scene area.
- If the Gazebo world has scene data and the websocket stream is compatible, the Gazebo scene renders in the canvas.

Open the browser developer console while testing. This app intentionally prints verbose diagnostics there, including package export details, WebSocket probe events, unhandled rendering errors, and `gzweb` status changes when available.

## Common Failure Modes

### Port `9002` Is Not Exposed

Symptom:

- The debug panel shows a raw WebSocket failure.
- The browser console may show connection refused.

Fix:

```bash
podman run --rm --name ros-minimal -p 9002:9002 localhost/eysip-ros2-workshop:minimal
```

If running through Kubernetes or a backend gateway, make sure the gateway proxies WebSocket upgrade requests, not just ordinary HTTP.

### Gazebo WebSocket Bridge Is Not Running

Symptom:

- Gazebo Sim may be running, but nothing is listening on port `9002`.

Check:

```bash
podman exec ros-minimal supervisorctl status
podman exec ros-minimal bash -lc 'tail -n 100 /tmp/gazebo-web.err.log'
```

Fix:

- Make sure the supervised `gazebo-web` process is running.
- Check `/tmp/gazebo-web.log` and `/tmp/gazebo-web.err.log`.
- Make sure the world includes `SceneBroadcaster` so there is scene data to stream.

### Wrong WebSocket URL

Symptom:

- `ws://localhost:9002` works only when the browser and published container port are on the same machine.
- In a remote dev machine, VM, cluster, or gateway setup, `localhost` may point to the browser user's machine instead of the workspace host.

Fix:

- Use the host or gateway URL that is reachable from the browser.
- Use `wss://...` when the cockpit is served over HTTPS.

### Browser CORS Or WebSocket Upgrade Issues

WebSocket connections are not governed by CORS in exactly the same way as `fetch`, but browsers still enforce mixed-content rules and proxies must support `Upgrade: websocket`.

Symptoms:

- The browser blocks `ws://...` from an `https://...` page.
- The gateway returns an HTTP error instead of `101 Switching Protocols`.
- The raw probe fails before `gzweb` can render.

Fix:

- Use `wss://` from HTTPS pages.
- Confirm the gateway preserves `Connection: Upgrade` and `Upgrade: websocket`.
- Avoid path rewriting surprises until the direct `ws://localhost:9002` test works.

### Package/API Mismatch With `gzweb`

Symptom:

- The debug panel says it could not find a `SceneManager` export.
- The package loads, but rendering does not start.
- The browser console shows module, Three.js, or WebGL errors.
- Vite reports that it cannot resolve Babel helpers from `three-nebula`, such as `@babel/runtime/helpers/defineProperty`.

Fix:

- Check the logged module export names in the browser console.
- Confirm the installed `gzweb` version matches the expected `SceneManager` API.
- Inspect `window.__gazeboSceneManager` in the browser console.
- Try reinstalling dependencies with a clean `node_modules` if package versions drift.
- Keep `@babel/runtime` as a direct dependency of this test app. `gzweb` depends on `three-nebula`, and the published `three-nebula` ESM files import Babel runtime helpers.

## Cockpit Integration Notes

These are the steps taken in this test app, written as implementation guidance for the modular student cockpit:

1. Create a dedicated Gazebo frame component with a stable container element. This test uses `#gzScene`.
2. Accept a WebSocket URL as configuration. In production, the URL should come from the backend/gateway, not from student input.
3. Treat the Gazebo frame as eventually available. The cockpit can mount before the websocket bridge or Gazebo world is ready and retry until the endpoint appears.
4. Probe the WebSocket endpoint before creating `SceneManager`. This keeps missing-port failures clear.
5. Load `gzweb` in the browser bundle and create a `SceneManager` with `{ elementId, websocketUrl }`.
6. Subscribe to `SceneManager` connection status when the package exposes `getConnectionStatusAsObservable()`.
7. Expose useful debug information during development: selected URL, websocket probe result, package exports, connection status, and rendering errors.
8. Handle cleanup when the cockpit frame unmounts: unsubscribe status observers and call `disconnect()`, `destroy()`, or `dispose()` if the `SceneManager` version exposes one of those methods.
9. Resize the viewer when the cockpit panel changes size. This test calls `sceneManager.resize()` on window resize; the cockpit should use a `ResizeObserver` on the frame/panel.
10. Route production traffic through the backend gateway as a WebSocket endpoint, likely something like `/ws/gazebo`, where the backend resolves the student's workspace from the session cookie.
11. Keep the Gazebo rendering frame separate from terminal/code frames. Student programs manipulate the sim through ROS 2/Gazebo APIs inside the workspace; this frame only visualizes the live scene.
12. Preserve a debug mode in the cockpit while the Gazebo Sim integration is being stabilized. Blank WebGL canvases are otherwise hard to diagnose.

For the current MVP architecture, the production cockpit should not ask students for `ws://localhost:9002`. It should receive a gateway URL from the backend, use `wss://` when the platform is served over HTTPS, and rely on the backend to authorize and proxy the WebSocket upgrade into the correct workspace pod.

## Assumptions

- Gazebo Sim may be launched after the viewer opens.
- The supervised `gazebo-web` bridge is running.
- The Gazebo world includes `SceneBroadcaster` or otherwise publishes scene data.
- Port `9002` is published to the host for local testing.
- The browser supports WebGL.
- The installed `gzweb` package exposes a browser-compatible `SceneManager` API.
