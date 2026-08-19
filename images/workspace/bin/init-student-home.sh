#!/usr/bin/env bash
set -euo pipefail

USER_NAME="${USER:-student}"
HOME_DIR="${HOME:-/home/${USER_NAME}}"
SEED_DIR="${STUDENT_HOME_SEED_DIR:-/opt/student-home-seed}"
MARKER="${HOME_DIR}/.podlab-home-seeded"
EXTENSIONS_MARKER="${HOME_DIR}/.podlab-code-server-extensions-installed"
CODE_SERVER_DATA_DIR="${HOME_DIR}/.local/share/code-server"
CODE_SERVER_EXTENSIONS_DIR="${CODE_SERVER_DATA_DIR}/extensions"
CODE_SERVER_EXTENSIONS_LIST="${CODE_SERVER_DATA_DIR}/extensions.txt"

install_code_server_extensions() {
  printf 'Installing extensions in background...\n'

  local extension
  while IFS= read -r extension || [ -n "${extension}" ]; do
    [ -z "${extension}" ] && continue

    if ! code-server \
      --user-data-dir "${CODE_SERVER_DATA_DIR}" \
      --extensions-dir "${CODE_SERVER_EXTENSIONS_DIR}" \
      --install-extension "${extension}"; then
      printf 'warning: failed to install code-server extension: %s\n' "${extension}" >&2
    fi
  done < "${CODE_SERVER_EXTENSIONS_LIST}"

  touch "${EXTENSIONS_MARKER}"
  printf 'Finished installing extensions.\n'
}

mkdir -p "${HOME_DIR}"

if [ -d "${SEED_DIR}" ]; then
  cp -rn "${SEED_DIR}/." "${HOME_DIR}/"
  touch "${MARKER}"
fi

mkdir -p \
  "${HOME_DIR}/dev_ws" \
  "${HOME_DIR}/.config/code-server" \
  "${CODE_SERVER_DATA_DIR}/User" \
  "${CODE_SERVER_EXTENSIONS_DIR}" \
  "${HOME_DIR}/.local/state/workspace-services"

if [ ! -f "${HOME_DIR}/.config/code-server/config.yaml" ]; then
  cat > "${HOME_DIR}/.config/code-server/config.yaml" <<EOF
bind-addr: 0.0.0.0:${CODE_SERVER_PORT:-7682}
auth: none
cert: false
user-data-dir: ${HOME_DIR}/.local/share/code-server
extensions-dir: ${HOME_DIR}/.local/share/code-server/extensions
EOF
fi

if [ -f "${CODE_SERVER_EXTENSIONS_LIST}" ] && [ ! -f "${EXTENSIONS_MARKER}" ] && curl -fsS --connect-timeout 2 --max-time 5 https://open-vsx.org >/dev/null 2>&1; then
  install_code_server_extensions &
fi

exec "$@"
