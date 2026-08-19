#!/usr/bin/env bash
set -euo pipefail

# Installs Selkies-GStreamer (WebRTC desktop streaming with NVENC) from the
# upstream release artifacts, replacing the previous Xpra stack. Mirrors the
# install block from selkies-project/docker-selkies-egl-desktop so the codec
# and GStreamer build are the upstream-tested combination rather than a
# hand-assembled one.

export DEBIAN_FRONTEND=noninteractive

SELKIES_VERSION="${SELKIES_VERSION:-1.6.2}"
UBUNTU_VERSION="${UBUNTU_VERSION:-24.04}"
SELKIES_ARCH="${SELKIES_ARCH:-amd64}"
SELKIES_VENV="${SELKIES_VENV:-/opt/selkies-venv}"

# Runtime deps for the Selkies GStreamer build, the virtual display, and the
# window manager. Several overlap with the existing GUI dependency layer; apt
# is idempotent so the duplicates are no-ops, and listing them here keeps this
# script self-contained.
APT_PACKAGES=(
  python3-dev
  python3-gi
  python3-venv
  libgcrypt20
  libgirepository-1.0-1
  glib-networking
  libglib2.0-0
  libgudev-1.0-0
  libvpx-dev
  x264
  x265
  libdrm2
  libegl1
  libgl1
  libopengl0
  libgles1
  libgles2
  libglvnd0
  libglx0
  mesa-utils
  xcvt
  libopenh264-dev
  svt-av1
  aom-tools
  wayland-protocols
  libwayland-dev
  libwayland-egl1
  dbus-x11
  xfce4-terminal
  xsel
  x11-utils
  x11-xkb-utils
  x11-xserver-utils
  xserver-xorg-core
  libx11-xcb1
  libxcb-dri3-0
  libxdamage1
  libxfixes3
  libxv1
  libxtst6
  libxext6
  xvfb
  icewm
)

apt-get update
apt-get install -y --no-install-recommends "${APT_PACKAGES[@]}"

# VirtualGL: renders OpenGL apps on the NVIDIA GPU (EGL backend) under the
# virtual X server when vglrun is used. Pinned for reproducibility; bump as
# needed.
VIRTUALGL_VERSION="${VIRTUALGL_VERSION:-3.1.4}"
curl -fL -o /tmp/virtualgl.deb \
  "https://github.com/VirtualGL/virtualgl/releases/download/${VIRTUALGL_VERSION}/virtualgl_${VIRTUALGL_VERSION}_amd64.deb"
apt-get install -y --no-install-recommends /tmp/virtualgl.deb
rm -f /tmp/virtualgl.deb

base_url="https://github.com/selkies-project/selkies/releases/download/v${SELKIES_VERSION}"

# 1. GStreamer build carrying the NVENC + WebRTC plugins. Expands into
#    /opt/gstreamer and provides /opt/gstreamer/gst-env (sourced by start-selkies).
curl -fsSL \
  "${base_url}/gstreamer-selkies_gpl_v${SELKIES_VERSION}_ubuntu${UBUNTU_VERSION}_${SELKIES_ARCH}.tar.gz" \
  | tar -xzf - -C /opt

# 2. Selkies Python package. websockets is pinned as upstream does to avoid an
#    incompatible major.
selkies_wheel="/tmp/selkies_gstreamer-${SELKIES_VERSION}-py3-none-any.whl"
curl -fL -o "${selkies_wheel}" \
  "${base_url}/selkies_gstreamer-${SELKIES_VERSION}-py3-none-any.whl"
python3 -m venv --system-site-packages "${SELKIES_VENV}"
"${SELKIES_VENV}/bin/python" -m pip install --no-cache-dir --upgrade pip setuptools wheel
"${SELKIES_VENV}/bin/python" -m pip install --no-cache-dir "${selkies_wheel}" "websockets<14.0"
rm -f "${selkies_wheel}"

# 3. HTML5 web client. Expands into /opt/gst-web; Selkies serves it directly on
#    its --port (no separate web server needed).
curl -fsSL \
  "${base_url}/selkies-gstreamer-web_v${SELKIES_VERSION}.tar.gz" \
  | tar -xzf - -C /opt

"${SELKIES_VENV}/bin/python" - <<'PY'
from importlib.util import find_spec
from pathlib import Path


def replace_once(text: str, old: str, new: str, label: str) -> str:
    if old not in text:
        raise SystemExit(f"Selkies no-audio patch failed: missing {label}")
    return text.replace(old, new, 1)


# LATENCY RESUME FIX - DISABLED FOR UPSTREAM WEB CLIENT PARITY
# Do not patch /opt/gst-web/app.js at this stage. The candidate fix below
# forced a fresh WebRTC negotiation when a browser tab returned after being
# hidden long enough for receiver or jitter-buffer state to become stale. It is
# intentionally kept as one disabled block so we can circle back without
# shipping a local Selkies web-client divergence.
#
# web_app_js = Path("/opt/gst-web/app.js")
# web_app_text = web_app_js.read_text()
# web_app_text = replace_once(
#     web_app_text,
#     '''// Actions to take whenever window changes focus
# window.addEventListener('focus', () => {
# ''',
#     '''// Force a fresh WebRTC negotiation when a browser throttled the tab long
# // enough to leave the receiver or jitter buffer in a high-latency state.
# var podlabHiddenSince = null;
# var podlabResumeResetMs = parseInt(localStorage.getItem("selkiesResumeResetMs") || "120000", 10);
# document.addEventListener("visibilitychange", () => {
#     if (document.hidden) {
#         podlabHiddenSince = Date.now();
#         return;
#     }
#     if (podlabHiddenSince !== null && Date.now() - podlabHiddenSince >= podlabResumeResetMs && app.status === "connected") {
#         webrtc._setStatus("Reconnecting after background tab pause");
#         signalling.disconnect();
#     }
#     podlabHiddenSince = null;
# });
#
# // Actions to take whenever window changes focus
# window.addEventListener('focus', () => {
# ''',
#     "visibility resume reconnect hook",
# )
# web_app_js.write_text(web_app_text)

spec = find_spec("selkies_gstreamer")
if spec is None or spec.submodule_search_locations is None:
    raise SystemExit("Selkies silent-audio patch failed: package not found")

pkg_dir = Path(next(iter(spec.submodule_search_locations)))
gstwebrtc_app_py = pkg_dir / "gstwebrtc_app.py"
gstwebrtc_text = gstwebrtc_app_py.read_text()

gstwebrtc_text = replace_once(
    gstwebrtc_text,
    """        # Create element for receiving audio from pulseaudio.
        pulsesrc = Gst.ElementFactory.make("pulsesrc", "pulsesrc")

        # Let the audio source provide the global clock.
        # This is important when trying to keep the audio and video
        # jitter buffers in sync. If there is skew between the video and audio
        # buffers, features like NetEQ will continuously increase the size of the
        # jitter buffer to catch up and will never recover.
        pulsesrc.set_property("provide-clock", True)

        # Apply stream time to buffers, this helps with pipeline synchronization.
        # Disabled by default because pulsesrc should not be re-timestamped with the current stream time when pushed out to the GStreamer pipeline and destroy the original synchronization.
        pulsesrc.set_property("do-timestamp", False)

        # Maximum and minimum amount of data to read in each iteration in microseconds
        pulsesrc.set_property("buffer-time", 100000)
        pulsesrc.set_property("latency-time", 1000)
""",
    """        # Keep the upstream browser protocol intact without capturing host audio.
        # The HTML client waits for an audio WebRTC peer before declaring the
        # session connected, so feed that peer synthetic silence instead of
        # depending on PulseAudio or any real audio device.
        pulsesrc = Gst.ElementFactory.make("audiotestsrc", "silent_audiosrc")
        pulsesrc.set_property("wave", "silence")
        pulsesrc.set_property("is-live", True)
        pulsesrc.set_property("do-timestamp", True)
""",
    "PulseAudio source replacement",
)
gstwebrtc_app_py.write_text(gstwebrtc_text)

PY

# 4. JS interposer (LD_PRELOAD library that maps app GL/X into the capture
#    path). Installed now; preloading is an optional tuning step, not required
#    for the first ximagesrc-based capture test.
curl -fL -o /tmp/selkies-js-interposer.deb \
  "${base_url}/selkies-js-interposer_v${SELKIES_VERSION}_ubuntu${UBUNTU_VERSION}_${SELKIES_ARCH}.deb"
apt-get install -y --no-install-recommends /tmp/selkies-js-interposer.deb
rm -f /tmp/selkies-js-interposer.deb

apt-get clean
rm -rf /var/lib/apt/lists/* /root/.cache/pip
