#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -eq 0 ]]; then
  echo "usage: run-with-virtualgl COMMAND [ARGS...]" >&2
  exit 64
fi

has_nvidia_gpu() {
  command -v nvidia-smi >/dev/null 2>&1 && nvidia-smi -L >/dev/null 2>&1
}

if [[ "${PODLAB_DISABLE_VIRTUALGL:-}" == "1" \
  || "${PODLAB_VIRTUALGL_ACTIVE:-}" == "1" ]] \
  || ! command -v vglrun >/dev/null 2>&1 \
  || ! has_nvidia_gpu; then
  exec "$@"
fi

export PODLAB_VIRTUALGL_ACTIVE=1
exec vglrun -d "${VGL_DISPLAY:-egl}" "$@"
