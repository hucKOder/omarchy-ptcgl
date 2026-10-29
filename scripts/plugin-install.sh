#!/bin/bash

set -euo pipefail

finish() {
  local status=$?
  if [[ -t 0 && -t 1 ]]; then
    read -r -p 'Press Enter to close this terminal…' _ || true
  fi
  exit "$status"
}

main() {
  if (( $# )); then
    if (( $# == 1 )) && [[ $1 == --help || $1 == -h ]]; then
      printf '%s\n' 'Install or upgrade the PTCGL launcher from its authenticated stable release.' \
        'Usage: bash scripts/plugin-install.sh' \
        'Game setup is left to the launcher. Run as your normal desktop user.'
      return
    fi
    printf 'Usage: bash scripts/plugin-install.sh\n' >&2
    return 2
  fi
  (( EUID != 0 )) || { printf 'Run as your desktop user, without sudo.\n' >&2; return 1; }
  local repo state status
  repo=$(dirname -- "$(dirname -- "$(realpath -- "${BASH_SOURCE[0]}")")")
  [[ -f $repo/bootstrap.sh ]] || { printf 'The plugin checkout is missing bootstrap.sh.\n' >&2; return 1; }
  state=${XDG_STATE_HOME:-$HOME/.local/state}
  [[ $state == /* ]] || { printf 'XDG_STATE_HOME must be an absolute path.\n' >&2; return 1; }
  # The app directory and its lock are private, including when ancestors are new.
  umask 077
  mkdir -p -- "$state/ptcgl-omarchy"
  exec 9>"$state/ptcgl-omarchy/companion-install.lock"
  if ! flock -n 9; then
    printf 'A companion launcher installation is already running.\n' >&2
    return 1
  fi
  printf '\nInstalling the launcher. Omarchy will request your sudo password if needed.\n'
  if bash "$repo/bootstrap.sh" --launcher-only; then
    status=0
    printf '\nLauncher installed. Reopen the PTCGL panel and choose Set up & play.\n'
  else
    status=$?
    printf '\nLauncher installation failed (exit %s). Review the output above and retry.\n' "$status" >&2
  fi
  flock -u 9
  return "$status"
}

# Parse the whole helper before a terminal prompt or a plugin checkout update.
if (( $# == 0 )); then trap finish EXIT; fi
main "$@"
