#!/usr/bin/env bash
set -euo pipefail

# Installs Selkies 2.0.0 (pure-Python WebRTC/WebSockets desktop streaming)
# from the upstream per-Ubuntu .deb. Replaces the 1.6.2 installer, whose
# release assets upstream deleted (GStreamer tarball, selkies_gstreamer
# wheel, js-interposer deb all 404 as of Sep 2026).
#
# 2.0.0 packaging differences that matter here:
# - No bundled GStreamer: capture/encode is pure Python (aiortc-style
#   selkies.webrtc), so there is no gst-env to source and no NVENC plugin
#   build. Encoder names are codecs (h264enc, vp8enc, ...), not GStreamer
#   elements (nvh264enc/x264enc).
# - Self-contained /opt/selkies venv (python3.12) + /usr/bin/selkies
#   entry point; the HTML5 client ships inside the package.
# - Audio off is a native flag (--audio-enabled=false); the 1.x
#   pulsesrc->audiotestsrc source patch has no target and is dropped.
# - Settings resolve CLI flag > SELKIES_<NAME> env > default.

export DEBIAN_FRONTEND=noninteractive

SELKIES_VERSION="${SELKIES_VERSION:-2.0.0}"
# Pinned to the noble-based base image (osrf/ros:jazzy-desktop-full-noble
# is Ubuntu 24.04). Revisit if the base moves.
SELKIES_DEB_OS="${SELKIES_DEB_OS:-ubuntu24.04}"
SELKIES_ARCH="${SELKIES_ARCH:-amd64}"

# Runtime deps: virtual X + window manager stack, the .deb's own Depends,
# and VirtualGL (no-op without an NVIDIA GPU today; kept for the GPU
# follow-up — run-with-virtualgl already falls back to direct exec).
APT_PACKAGES=(
  xvfb
  icewm
  xfce4-terminal
  dbus-x11
  x11-utils
  x11-xkb-utils
  x11-xserver-utils
  xserver-xorg-core
  libx11-xcb1
  mesa-utils
  python3
  libpulse0
  libxcb1
  libxkbcommon0
  libva2
  libva-drm2
  libva-x11-2
  libdrm2
  libgbm1
  libegl1
  libwayland-server0
  libglib2.0-0
  libpixman-1-0
  libxcb-render0
  libxcb-shm0
  libxcb-dri3-0
  libxfixes3
  libxext6
  libice6
  libsm6
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

base_url="https://github.com/selkies-project/selkies/releases/download/${SELKIES_VERSION}"
selkies_deb="/tmp/selkies_${SELKIES_VERSION}.deb"
curl -fL -o "${selkies_deb}" \
  "${base_url}/selkies-${SELKIES_VERSION}-${SELKIES_DEB_OS}-${SELKIES_ARCH}.deb"
apt-get install -y --no-install-recommends "${selkies_deb}"
rm -f "${selkies_deb}"

command -v selkies >/dev/null 2>&1 || { echo "selkies install failed: no /usr/bin/selkies" >&2; exit 1; }
selkies --version

apt-get clean
rm -rf /var/lib/apt/lists/* /root/.cache/pip
