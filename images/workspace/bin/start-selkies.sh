#!/usr/bin/env bash
set -euo pipefail

# Launches the Selkies-GStreamer desktop stream. Unlike Xpra (which started its
# own display server), Selkies attaches to an existing X display, so this script
# brings up a virtual X server and a window manager first, then execs Selkies.

display="${SELKIES_DISPLAY:-${DISPLAY:-:50}}"
case "${display}" in
  :*) ;;
  *) display=":${display}" ;;
esac
export DISPLAY="${display}"
export TZ="${TZ:-Asia/Kolkata}"

# Bring in the Selkies-bundled GStreamer (NVENC + WebRTC plugins).
if [[ -f /opt/gstreamer/gst-env ]]; then
  # shellcheck disable=SC1091
  . /opt/gstreamer/gst-env
fi

export SELKIES_WEB_ROOT="${SELKIES_WEB_ROOT:-/opt/gst-web}"
SELKIES_VENV="${SELKIES_VENV:-/opt/selkies-venv}"
selkies_bin="${SELKIES_VENV}/bin/selkies-gstreamer"
if [[ ! -x "${selkies_bin}" ]]; then
  echo "Selkies executable not found at ${selkies_bin}" >&2
  exit 1
fi
export PATH="${SELKIES_VENV}/bin:${PATH}"

has_nvidia_gpu() {
  command -v nvidia-smi >/dev/null 2>&1 && nvidia-smi -L >/dev/null 2>&1
}

if [[ -z "${XDG_RUNTIME_DIR:-}" || ! -d "${XDG_RUNTIME_DIR}" || ! -w "${XDG_RUNTIME_DIR}" ]]; then
  export XDG_RUNTIME_DIR="/tmp/runtime-${USER:-student}"
fi
mkdir -p "${XDG_RUNTIME_DIR}"
chmod 700 "${XDG_RUNTIME_DIR}"

screen_geometry="${SELKIES_SCREEN:-1440x900x24}"
display_num="${display#:}"
x_socket="/tmp/.X11-unix/X${display_num}"

export VGL_DISPLAY="${VGL_DISPLAY:-egl}"
gpu_available=false
if has_nvidia_gpu; then
  gpu_available=true
fi

mkdir -p /tmp/.X11-unix
chmod 1777 /tmp/.X11-unix 2>/dev/null || true

# Start a virtual X server if one is not already present on this display.
# Xvfb provides the desktop surface. When NVIDIA is attached, VirtualGL uses the
# EGL backend for GL apps; otherwise apps fall back to Mesa/software rendering.
if [[ ! -S "${x_socket}" ]]; then
  echo "Starting Xvfb on ${display} (${screen_geometry})"
  Xvfb "${display}" -screen 0 "${screen_geometry}" -dpi "${SELKIES_DPI:-96}" \
    +extension COMPOSITE +extension DAMAGE +extension GLX +extension RANDR \
    +extension RENDER +extension MIT-SHM +extension XFIXES +extension XTEST \
    +iglx +render -nolisten tcp -ac -noreset -shmem &
fi

for _ in $(seq 1 50); do
  [[ -S "${x_socket}" ]] && break
  sleep 0.2
done
if [[ ! -S "${x_socket}" ]]; then
  echo "X server did not come up on ${display}" >&2
  exit 1
fi

icewm_config_dir="${ICEWM_CONFIG_DIR:-${HOME}/.icewm}"
icewm_preferences_src="${ICEWM_PREFERENCES:-/usr/local/share/podlab/icewm-preferences}"
icewm_prefoverride_src="${ICEWM_PREFOVERRIDE:-/usr/local/share/podlab/icewm-prefoverride}"
icewm_preferences="${icewm_config_dir}/preferences"
icewm_prefoverride="${icewm_config_dir}/prefoverride"
icewm_wallpaper="${ICEWM_WALLPAPER:-/usr/local/share/podlab/wallpaper.jpg}"
icewm_theme="${ICEWM_THEME:-}"
mkdir -p "${icewm_config_dir}"
export ICEWM_PRIVCFG="${icewm_config_dir}"
if [[ -f "${icewm_preferences_src}" ]]; then
  cp "${icewm_preferences_src}" "${icewm_preferences}"
fi
if [[ -f "${icewm_prefoverride_src}" ]]; then
  cp "${icewm_prefoverride_src}" "${icewm_prefoverride}"
fi
touch "${icewm_config_dir}/menu" "${icewm_config_dir}/programs"
cat >"${icewm_config_dir}/toolbar" <<'EOF'
prog "Terminal" utilities-terminal xfce4-terminal --working-directory=/home/student/dev_ws
EOF
if [[ -n "${icewm_theme}" ]]; then
  printf 'Theme="%s"\n' "${icewm_theme}" >"${icewm_config_dir}/theme"
else
  rm -f "${icewm_config_dir}/theme"
fi
cat >>"${icewm_prefoverride}" <<EOF
DesktopBackgroundCenter=0
DesktopBackgroundScaled=1
DesktopBackgroundColor="rgb:1f/23/29"
DesktopBackgroundImage="${icewm_wallpaper}"
ShuffleBackgroundImages=0
CycleBackgroundsPeriod=0
WinMenuItems=""
EOF

if command -v icewmbg >/dev/null 2>&1; then
  if [[ -s "${icewm_wallpaper}" ]]; then
    echo "Applying IceWM wallpaper from ${icewm_wallpaper}"
    icewmbg --replace --config="${icewm_preferences}" --image="${icewm_wallpaper}" --scaled=1 --center=0 &
  else
    echo "IceWM wallpaper missing or empty at ${icewm_wallpaper}; using solid background"
    icewmbg --replace --config="${icewm_preferences}" --color="rgb:1f/23/29" --scaled=1 --center=0 &
  fi
fi

if command -v icewm-session >/dev/null 2>&1; then
  echo "Starting IceWM session on ${display}"
  if command -v dbus-launch >/dev/null 2>&1; then
    /usr/local/bin/run-with-virtualgl dbus-launch --exit-with-session icewm-session --nobg --config="${icewm_preferences}" &
  else
    /usr/local/bin/run-with-virtualgl icewm-session --nobg --config="${icewm_preferences}" &
  fi
elif command -v icewm >/dev/null 2>&1; then
  echo "Starting IceWM on ${display}"
  /usr/local/bin/run-with-virtualgl icewm --config="${icewm_preferences}" &
else
  echo "IceWM is not installed; continuing without a window manager" >&2
fi

requested_encoder="${SELKIES_ENCODER:-auto}"
case "${requested_encoder}" in
  auto)
    if [[ "${gpu_available}" == "true" ]]; then
      encoder="nvh264enc"
    else
      encoder="x264enc"
    fi
    ;;
  *)
    encoder="${requested_encoder}"
    ;;
esac
export SELKIES_ENCODER="${encoder}"

port="${SELKIES_PORT:-8080}"

# Authentication and TLS are owned by the upstream gateway (later phase); keep
# them off here so a direct browser hit works during local testing.
selkies_args=(
  --addr=0.0.0.0
  --port="${port}"
  --web_root="${SELKIES_WEB_ROOT}"
  --enable_https=false
  --enable_basic_auth=false
  --turn_host="${SELKIES_TURN_HOST:-}"
  --turn_port="${SELKIES_TURN_PORT:-}"
  --turn_shared_secret="${SELKIES_TURN_SHARED_SECRET:-}"
  --turn_username="${SELKIES_TURN_USERNAME:-}"
  --turn_password="${SELKIES_TURN_PASSWORD:-}"
  --turn_protocol="${SELKIES_TURN_PROTOCOL:-udp}"
  --turn_tls="${SELKIES_TURN_TLS:-false}"
  --encoder="${encoder}"
  --video_bitrate="${SELKIES_VIDEO_BITRATE:-3000}"
  --framerate="${SELKIES_FRAMERATE:-30}"
  --audio_bitrate="${SELKIES_AUDIO_BITRATE:-6000}"
  --audio_channels="${SELKIES_AUDIO_CHANNELS:-1}"
  --enable_clipboard="${SELKIES_ENABLE_CLIPBOARD:-true}"
  --enable_resize="${SELKIES_ENABLE_RESIZE:-true}"
  # STUN is always on: ICE gathers host/server-reflexive candidates, so a
  # same-network (campus LAN / localhost) client connects directly with no relay.
  --stun_host="${SELKIES_STUN_HOST:-stun.l.google.com}"
  --stun_port="${SELKIES_STUN_PORT:-19302}"
)

# A local test can pass direct TURN host/secret settings. The later production
# path should prefer REST-minted, short-lived credentials instead.
if [[ -n "${SELKIES_TURN_REST_URI:-}" ]]; then
  selkies_args+=(--turn_rest_uri="${SELKIES_TURN_REST_URI}")
  selkies_args+=(--turn_rest_username="${SELKIES_TURN_REST_USERNAME:-selkies-${HOSTNAME:-workspace}}")
  selkies_args+=(--turn_rest_username_auth_header="${SELKIES_TURN_REST_USERNAME_AUTH_HEADER:-x-auth-user}")
  selkies_args+=(--turn_rest_protocol_header="${SELKIES_TURN_REST_PROTOCOL_HEADER:-x-turn-protocol}")
  selkies_args+=(--turn_rest_tls_header="${SELKIES_TURN_REST_TLS_HEADER:-x-turn-tls}")
fi

echo "Starting selkies-gstreamer on ${display} port ${port} (encoder=${encoder}, web_root=${SELKIES_WEB_ROOT})"
exec "${selkies_bin}" "${selkies_args[@]}"
