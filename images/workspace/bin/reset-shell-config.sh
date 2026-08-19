#!/usr/bin/env bash
set -euo pipefail

user_name="${USER:-student}"
home_dir="${HOME:-/home/${user_name}}"
seed_bashrc="${STUDENT_HOME_SEED_DIR:-/opt/student-home-seed}/.bashrc"
target_bashrc="${home_dir}/.bashrc"

if [[ ! -f "${seed_bashrc}" ]]; then
  echo "reset-shell-config: seed .bashrc not found at ${seed_bashrc}" >&2
  exit 1
fi

mkdir -p "${home_dir}"

if [[ -f "${target_bashrc}" ]] && ! cmp -s "${seed_bashrc}" "${target_bashrc}"; then
  backup="${target_bashrc}.backup.$(date +%Y%m%d-%H%M%S)"
  cp "${target_bashrc}" "${backup}"
  echo "Backed up existing .bashrc to ${backup}"
fi

cp "${seed_bashrc}" "${target_bashrc}"
echo "Restored ${target_bashrc} from ${seed_bashrc}"
echo "Open a new terminal tab, or run: source ~/.bashrc"
