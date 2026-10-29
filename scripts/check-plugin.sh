#!/bin/bash

set -euo pipefail

repo=$(dirname -- "$(dirname -- "$(realpath -- "${BASH_SOURCE[0]}")")")
omarchy_path=${OMARCHY_PATH:-/usr/share/omarchy}
qmllint=${QMLLINT:-$(command -v qmllint || true)}
if [[ -z $qmllint && -x /usr/lib/qt6/bin/qmllint ]]; then
  qmllint=/usr/lib/qt6/bin/qmllint
fi
[[ -n $qmllint && -d $omarchy_path/shell ]] || {
  printf 'Omarchy Quattro and Qt qmllint are required for make check-plugin.\n' >&2
  exit 1
}
omarchy plugin validate "$repo"

# Quickshell maps qs.* to its config root at runtime. Give standalone qmllint
# that same import layout without modifying Omarchy or the plugin checkout.
work_dir=$(mktemp -d "${TMPDIR:-/tmp}/ptcgl-plugin-lint.XXXXXXXX")
trap 'rm -rf -- "$work_dir"' EXIT
ln -s -- "$omarchy_path/shell" "$work_dir/qs"
"$qmllint" --max-warnings 0 -I "$work_dir" -I "$omarchy_path/shell" \
  "$repo/plugin/BarWidget.qml" "$repo/plugin/Panel.qml"
