#!/usr/bin/env bash
set -euo pipefail

export DISPLAY="${DISPLAY:-:50}"
export VGL_DISPLAY="${VGL_DISPLAY:-egl}"

exec /usr/local/bin/run-with-virtualgl code-server \
  --config "/home/${USER:-student}/.config/code-server/config.yaml" \
  --bind-addr "0.0.0.0:${CODE_SERVER_PORT:-7682}" \
  "${WORKSPACE_DIR:-/home/student/dev_ws}"
