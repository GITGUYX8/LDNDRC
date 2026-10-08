#!/usr/bin/env bash
set -euo pipefail

# Launches the Selkies 2.0.0 desktop stream. Selkies attaches to an existing
# X display, so this script brings up a virtual X server and a window manager
# first, then execs Selkies. (2.0.0 is pure Python — no GStreamer env to
# source, no venv binary; /usr/bin/selkies comes from the .deb.)

display="${SELKIES_DISPLAY:-${DISPLAY:-:50}}"
case "${display}" in
  :*) ;;
  *) display=":${display}" ;;
esac
export DISPLAY="${display}"
export TZ="${TZ:-Asia/Kolkata}"

selkies_bin="$(command -v selkies || true)"
if [[ -z "${selkies_bin}" ]]; then
  echo "Selkies executable not found on PATH" >&2
  exit 1
fi

if [[ -z "${XDG_RUNTIME_DIR:-}" || ! -d "${XDG_RUNTIME_DIR}" || ! -w "${XDG_RUNTIME_DIR}" ]]; then
  export XDG_RUNTIME_DIR="/tmp/runtime-${USER:-student}"
fi
mkdir -p "${XDG_RUNTIME_DIR}"
chmod 700 "${XDG_RUNTIME_DIR}"

screen_geometry="${SELKIES_SCREEN:-1440x900x24}"
display_num="${display#:}"
x_socket="/tmp/.X11-unix/X${display_num}"

export VGL_DISPLAY="${VGL_DISPLAY:-egl}"

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

# 2.0.0 encoder names are codecs (h264enc, vp8enc, ...), not GStreamer
# elements. `auto` omits the flag (upstream default h264enc, software
# without a GPU); anything else passes through and 2.0.0 validates it.
requested_encoder="${SELKIES_ENCODER:-auto}"
selkies_encoder_args=()
if [[ "${requested_encoder}" != "auto" ]]; then
  selkies_encoder_args=(--encoder="${requested_encoder}")
fi

port="${SELKIES_PORT:-8080}"

# Authentication and TLS are owned by the upstream gateway (later phase); keep
# them off here so a direct browser hit works during local testing. Audio is
# off natively (replaces the 1.x pulsesrc source patch). Transport stays on
# the upstream default (websockets); pass SELKIES_MODE=webrtc to try WebRTC.
selkies_args=(
  --addr=0.0.0.0
  --port="${port}"
  --enable-https=false
  --enable-basic-auth=false
  --audio-enabled=false
  --enable-clipboard="${SELKIES_ENABLE_CLIPBOARD:-true}"
  --enable-resize="${SELKIES_ENABLE_RESIZE:-true}"
  --video-bitrate="${SELKIES_VIDEO_BITRATE:-3000}"
  --framerate="${SELKIES_FRAMERATE:-30}"
  --turn-host="${SELKIES_TURN_HOST:-}"
  --turn-port="${SELKIES_TURN_PORT:-}"
  --turn-shared-secret="${SELKIES_TURN_SHARED_SECRET:-}"
  --turn-username="${SELKIES_TURN_USERNAME:-}"
  --turn-password="${SELKIES_TURN_PASSWORD:-}"
  --turn-protocol="${SELKIES_TURN_PROTOCOL:-udp}"
  --turn-tls="${SELKIES_TURN_TLS:-false}"
  # STUN is always on: ICE gathers host/server-reflexive candidates, so a
  # same-network (campus LAN / localhost) client connects directly with no relay.
  --stun-host="${SELKIES_STUN_HOST:-stun.l.google.com}"
  --stun-port="${SELKIES_STUN_PORT:-19302}"
)

# Transport stays on the upstream default (websockets) unless overridden.
if [[ -n "${SELKIES_MODE:-}" ]]; then
  selkies_args+=(--mode="${SELKIES_MODE}")
fi
if [[ "${#selkies_encoder_args[@]}" -gt 0 ]]; then
  selkies_args+=("${selkies_encoder_args[@]}")
fi

# A local test can pass direct TURN host/secret settings. The later production
# path should prefer REST-minted, short-lived credentials instead.
if [[ -n "${SELKIES_TURN_REST_URI:-}" ]]; then
  selkies_args+=(--turn-rest-uri="${SELKIES_TURN_REST_URI}")
  selkies_args+=(--turn-rest-username="${SELKIES_TURN_REST_USERNAME:-selkies-${HOSTNAME:-workspace}}")
  selkies_args+=(--turn-rest-username-auth-header="${SELKIES_TURN_REST_USERNAME_AUTH_HEADER:-x-auth-user}")
  selkies_args+=(--turn-rest-protocol-header="${SELKIES_TURN_REST_PROTOCOL_HEADER:-x-turn-protocol}")
  selkies_args+=(--turn-rest-tls-header="${SELKIES_TURN_REST_TLS_HEADER:-x-turn-tls}")
fi

echo "Starting selkies 2.0.0 on ${display} port ${port} (encoder=${requested_encoder})"
exec "${selkies_bin}" "${selkies_args[@]}"
