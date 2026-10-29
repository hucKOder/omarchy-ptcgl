#!/bin/bash

set -euo pipefail

(( $# == 0 )) || { printf 'Usage: bash scripts/plugin-local.sh\n' >&2; exit 2; }
(( EUID != 0 )) || { printf 'Install the companion as your desktop user.\n' >&2; exit 1; }
repo=$(dirname -- "$(dirname -- "$(realpath -- "${BASH_SOURCE[0]}")")")
id=io.github.huckoder.ptcgl
# The Quattro plugin registry currently uses HOME/.config, independent of XDG.
config="$HOME/.config/omarchy"
destination="$config/plugins/$id"
[[ ! -e $destination && ! -L $destination ]] || {
  printf 'A plugin already exists at %s. Remove it with omarchy plugin remove %s before installing a fresh local copy.\n' "$destination" "$id" >&2
  exit 1
}
omarchy plugin validate "$repo"
mkdir -p -- "$config/plugins"
work_dir=$(mktemp -d "$config/.ptcgl-companion.XXXXXXXX")
trap 'rm -rf -- "$work_dir"' EXIT
for file in manifest.json README.md LICENSE bootstrap.sh plugin/BarWidget.qml plugin/Panel.qml plugin/Model.js scripts/plugin-install.sh docs/PLUGIN.md; do
  install -D -m 644 -- "$repo/$file" "$work_dir/$file"
done
mv -- "$work_dir" "$destination"
omarchy-shell shell rescanPlugins
for _ in 1 2 3 4 5; do
  if omarchy plugin list --json | jq -e --arg id "$id" 'any(.[]; .id == $id)' >/dev/null; then
    omarchy plugin enable "$id"
    printf 'Installed local PTCGL Companion at %s\n' "$destination"
    exit 0
  fi
  sleep 1
done
printf 'Copied the companion to %s; enable it once the Omarchy shell is running.\n' "$destination" >&2
exit 1
