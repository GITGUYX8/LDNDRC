# LDNDRC — Frontend UI/UX Design Plan

Status: **design draft** (pre-implementation). This document defines the user-facing
experience of the LDNDRC platform before any frontend code is written. It takes
structural inspiration from a reference implementation (React SPA with
Auth → Workspace → Cockpit flow) but deliberately changes the appearance,
navigation model, and interaction patterns.

---

## 1. Product framing

LDNDRC gives each student a private, browser-based ROS 2 + Gazebo workspace.
The frontend is the *only* thing a student ever sees — the cluster, pods, and
images are invisible. The UI must therefore answer three questions instantly:

1. **Where am I?** — a branded, recognizable product, not three raw tools.
2. **Is my workspace ready?** — honest, live status during provisioning.
3. **What can I do now?** — one obvious primary action at every state.

### The three tools (unchanged, browser-native)

| Tool | Route | Technology | Role |
|---|---|---|---|
| Code | `/code` | code-server (iframe) | VS Code in the browser |
| Sim | `/sim` | gzweb viewer (`sim-web-app`) | 3D Gazebo scene over websocket |
| Desktop | `/desktop` | Selkies stream (iframe/new tab) | Full Linux desktop (GUI tools: RViz, rqt) |

The frontend's job is to **wrap** these three tools in one coherent product
shell — not to rebuild them.

---

## 2. Design principles (where we diverge from the reference)

The reference uses a floating-window "cockpit" desktop metaphor (draggable,
overlapping windows with z-ordering). We deliberately move away from that:

1. **Focus over window-management.** Students are here to learn ROS, not to
   arrange windows. We use a **single active tool with a persistent rail**
   instead of draggable overlapping windows. One tool fills the stage; switching
   is one click and instant (tools stay alive in the background).
2. **Split-view as a first-class mode, not freeform drag.** Instead of arbitrary
   window placement, we offer exactly three layouts: `Code`, `Sim`, and
   `Code | Sim` (50/50 split, draggable divider). This covers 95% of real
   workflows (write launch file → watch the robot move) with zero fiddling.
3. **Status is ambient, not a page.** Session state lives in a persistent top
   bar (dot + elapsed time + stop button), visible from every screen. The
   reference hides status behind polling pages; we make it always-on.
4. **Desktop is an escape hatch, not a peer.** The full Selkies desktop opens in
   a new browser tab (it needs keyboard/mouse capture). The rail shows it as a
   secondary action, keeping the main stage for Code + Sim.
5. **Light-on-dark "mission control" identity.** Dark theme, but warmer and
   higher-contrast than the reference's teal-on-black. Accent color and
   typography chosen for long sessions (reduced eye strain, clear focus rings).

---

## 3. Information architecture

```
/                     → redirect to /launch (or /login if unauthenticated)
/login                → sign in (JWT via Go control panel)
/launch               → workspace status + Launch/Stop + tool cards
/app                  → the workspace shell (requires READY session)
    ?view=code        → code-server fills the stage        (default)
    ?view=sim         → gzweb viewer fills the stage
    ?view=split       → code | sim side by side
/desktop              → Selkies stream (full-page, own tab)
```

Admin views are out of scope for this phase (the Go API has the seams; a simple
admin table can be added later without changing the student IA).

### State machine (student session)

```
LOGGED_OUT → LOGGED_IN → NO_SESSION → PROVISIONING → READY ⇄ STOPPING → STOPPED
                              ↑                         ↓
                              └──────── ERROR ──────────┘
```

Every state has exactly one primary action:

| State | Primary action | Secondary |
|---|---|---|
| LOGGED_OUT | Sign in | — |
| NO_SESSION | **Launch workspace** | — |
| PROVISIONING | (disabled, live stepper) | Cancel |
| READY | **Open workspace** (`/app`) | Stop |
| STOPPED | **Restart workspace** | — |
| ERROR | Retry | View details (cluster events excerpt) |

---

## 4. Screen-by-screen design

### 4.1 Login (`/login`)

- Centered card on a dark canvas with a subtle grid/glow backdrop.
- Brand mark + name + one-line tagline: *"ROS 2 & Gazebo in your browser."*
- Email + password, inline validation, show/hide password toggle.
- Error banner for bad credentials; no page reload (fetch → JWT → store).
- No sign-up in this phase (accounts are provisioned by the instructor).

### 4.2 Launch (`/launch`)

The "mission control" home. Layout: brand header, then a centered status panel.

- **NO_SESSION:** large illustration-free hero — a single glowing
  **Launch workspace** button, with a one-line explainer of what happens
  ("We start a private ROS 2 Jazzy + Gazebo Harmonic environment just for you").
- **PROVISIONING:** a vertical **stepper** with live states, mapped from the
  Go API's session/workload status (same progression the reference computes):
  1. Creating your session
  2. Scheduling the workspace pod
  3. Pulling image & starting containers
  4. Starting ROS services
  5. Ready
  Each step: spinner → check. A subtle elapsed timer. Cancel button.
- **READY:** three tool cards (Code / Sim / Desktop) with one-line
  descriptions, plus **Open workspace** (goes to `/app`). A muted **Stop**
  button with confirm dialog.
- **ERROR:** red-accent panel, human message, raw detail in a collapsible
  `<details>` block, Retry button.

### 4.3 Workspace shell (`/app`) — the core UX

```
┌──────────────────────────────────────────────────────────────┐
│ ◆ LDNDRC   [Code] [Sim] [Split]      ● Ready  00:12:43  ⏻ ▤ │  ← top bar (48px)
├────┬─────────────────────────────────────────────────────────┤
│    │                                                         │
│ r  │                                                         │
│ a  │                  ACTIVE TOOL STAGE                      │
│ i  │         (code-server iframe / gzweb canvas /            │
│ l  │              split: code | sim with divider)            │
│    │                                                         │
└────┴─────────────────────────────────────────────────────────┘
```

**Top bar (persistent, every screen in `/app`):**
- Left: brand mark + view switcher (segmented control: Code · Sim · Split).
- Right: session status dot (green pulse = Ready), elapsed session timer,
  Desktop (opens new tab), Stop (confirm), user menu (sign out).

**Rail (left, 56px, icon-only):**
- Code, Sim, Split, Desktop icons — same actions as the view switcher,
  duplicated for muscle memory; plus a "Reconnect" action if a stream drops.

**Stage:**
- `code`: code-server iframe, full-bleed.
- `sim`: gzweb viewer (our `sim-web-app` build) full-bleed, with a small
  floating toolbar: reset camera, pause/play sim, reconnect.
- `split`: code left, sim right, draggable divider (persisted to
  `localStorage`). Both iframes stay mounted when switching views so
  code-server and the gz websocket never reconnect on view change
  (CSS visibility toggling, not unmount).

**Connection loss:** if any iframe/websocket drops, a non-blocking toast +
rail badge offers **Reconnect**; the stage shows a dimmed overlay with a
single retry button only after 3 failed attempts.

### 4.4 Desktop (`/desktop`)

- Opens in a new tab; minimal wrapper page with the Selkies stream full-bleed
  and a floating "Back to workspace" pill. Keyboard capture warning shown once.

---

## 5. Visual identity

### Theme: "Mission Control" (dark)

| Token | Value | Use |
|---|---|---|
| `--bg` | `#0b0e14` | page background |
| `--panel` | `#12161f` | cards, rail, top bar |
| `--panel-raised` | `#1a1f2b` | hover/active surfaces |
| `--line` | `#262d3d` | borders, dividers |
| `--ink` | `#e6e9f0` | primary text |
| `--muted` | `#8b93a7` | secondary text |
| `--accent` | `#7c6cf5` (violet) | primary actions, active states |
| `--accent-soft` | `rgba(124,108,245,.14)` | selected backgrounds |
| `--ok` | `#3ecf8e` | ready/status dot |
| `--warn` | `#f5b86c` | provisioning |
| `--danger` | `#f56c6c` | stop/error |

Rationale: violet accent distinguishes LDNDRC from the reference's teal and
from code-server's blue, while staying colorblind-safe against the green/red
status pair. Status colors are reserved *only* for session state — never for
decoration.

### Typography

- UI: **Inter** (system fallback stack).
- Numeric/timer/code-adjacent: **JetBrains Mono** (matches code-server's
  default mono, so the shell and the IDE feel like one product).

### Motion

- 150–200ms ease-out for view switches and hovers; no spring physics.
- Status dot: 2s pulse while PROVISIONING, steady glow when READY.
- Stepper: check-draw animation on step completion.
- Respect `prefers-reduced-motion`: disable pulse and transitions.

### Accessibility baseline

- Full keyboard navigation (rail and view switcher are real `<button>`s).
- Visible focus rings (`--accent` outline, 2px).
- Contrast ≥ 4.5:1 for all text tokens against their backgrounds.
- Status never conveyed by color alone (dot + text label).

---

## 6. Technical approach

| Decision | Choice | Why |
|---|---|---|
| Framework | **React + Vite + TypeScript** (SPA) | gzweb embeds cleanly as a component; reference proves the pattern; small surface (4 routes) |
| Serving | Static build embedded in the Go control panel via `embed.FS`, served at `/` | one binary, one container, no nginx sidecar |
| Auth | JWT from `POST /api/auth/login`, stored in memory + `sessionStorage`; `Authorization: Bearer` on API calls | matches existing Go `internal/auth` |
| API | Existing Go endpoints: `/api/auth/login`, `/api/sessions/*` (client-go seam) | no backend changes needed for UI v1 |
| Tool embedding | iframes for code-server & Selkies; gzweb as a **mounted component** (not iframe) using our `sim-web-app` code | gzweb as component avoids iframe websocket issues and lets us style the toolbar natively |
| State | React context (auth) + local component state; `localStorage` only for split ratio & dismissed hints | no Redux/Zustand needed at this size |
| Styling | Plain CSS with custom properties (tokens above), BEM-ish class names | matches reference pattern (single `styles.css`), zero dependency weight |

### Frontend repo layout (planned)

```
control-panel/web/
├── index.html
├── package.json
├── vite.config.ts
├── src/
│   ├── main.tsx
│   ├── App.tsx              # routes + auth gate
│   ├── api.ts               # fetch wrapper w/ JWT
│   ├── auth.tsx             # context + login page
│   ├── pages/
│   │   ├── Launch.tsx       # status + stepper + tool cards
│   │   ├── Workspace.tsx    # shell: top bar + rail + stage
│   │   └── Desktop.tsx      # Selkies wrapper
│   ├── components/
│   │   ├── TopBar.tsx
│   │   ├── Rail.tsx
│   │   ├── Stepper.tsx
│   │   ├── SplitStage.tsx   # draggable divider
│   │   └── GazeboScene.tsx  # gzweb mount (from sim-web-app)
│   └── styles.css           # tokens + all styles
└── embed.go                 # //go:embed dist
```

---

## 7. What we take from the reference (and what we change)

| Reference pattern | Verdict | Our change |
|---|---|---|
| Auth page with inline validation, show/hide password | **Keep pattern** | Restyle to Mission Control theme; drop sign-up |
| Workspace page with provision → poll → ready flow | **Keep flow** | Replace text messages with visual stepper; add Cancel |
| `workspaceProgressMessage()` status mapping | **Keep logic** | Drive stepper steps from the same states |
| Cockpit: draggable overlapping windows, z-order | **Replace** | Single-stage + segmented views + fixed 50/50 split |
| Desktop opens in new tab | **Keep** | Add minimal wrapper with "Back" pill |
| Shell header with brand + nav + account | **Keep pattern** | Persistent top bar with ambient status (dot + timer) |
| Teal-on-black palette, Inter | **Change** | Violet accent, warmer darks, JetBrains Mono for timers |
| Admin page (user/session cards) | **Defer** | API seams exist; UI in a later phase |
| React SPA served by nginx container | **Change** | Embedded in Go binary via `embed.FS` |

---

## 8. Phasing

| Phase | Deliverable | Depends on |
|---|---|---|
| **UI-1 (this phase)** | `control-panel/web/` scaffold: Login + Launch (stepper) + Workspace shell with Code/Sim/Split views, embedded in Go binary | Task 5 manifests (routes), Go control panel (done) |
| **UI-2** | Desktop wrapper page, reconnect toasts, split-ratio persistence, `prefers-reduced-motion` pass | UI-1 |
| **UI-3** | Admin table, instructor view, session metrics | client-go wired (Task 8+) |

## 9. Open questions

1. **Brand name/mark** — "LDNDRC" wordmark with a `◆` glyph as placeholder; final logo TBD.
2. **Sim toolbar controls** — which gzweb controls do we expose (reset camera, pause, stats)? Start minimal: reset camera + reconnect.
3. **Session idle timeout UX** — do we warn before auto-stop? (Backend policy first, UI banner second.)
4. **Mobile/tablet** — out of scope for UI-1 (desktop-first, min-width 1024px with a polite "use a larger screen" notice below that).
