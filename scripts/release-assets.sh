#!/bin/bash

set -euo pipefail

(( $# == 2 )) || { printf 'Usage: bash scripts/release-assets.sh <vX.Y.Z> <source-commit>\n' >&2; exit 1; }
release=$1
source_commit=$2
[[ $release =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || { printf 'Invalid release tag.\n' >&2; exit 1; }
[[ $source_commit =~ ^[0-9a-f]{40}$ ]] || { printf 'A full lowercase source commit is required.\n' >&2; exit 1; }
[[ -f install.sh && -f build/ptcgl-omarchy && -f scripts/release-notes.md.in ]] || { printf 'Run from the repository root after building the launcher.\n' >&2; exit 1; }

mkdir -p build/release
cp build/ptcgl-omarchy build/release/ptcgl-omarchy-linux-amd64
cp install.sh build/release/install.sh
launcher_hash=$(sha256sum build/release/ptcgl-omarchy-linux-amd64)
installer_hash=$(sha256sum build/release/install.sh)
jq -n --arg release "$release" --arg commit "$source_commit" \
  --arg launcher_hash "${launcher_hash%% *}" --arg installer_hash "${installer_hash%% *}" '{
    schema: 1,
    repository: "hucKOder/omarchy-ptcgl",
    release: $release,
    source_commit: $commit,
    platform: "linux-amd64",
    launcher: "ptcgl-omarchy-linux-amd64",
    launcher_sha256: $launcher_hash,
    installer_sha256: $installer_hash
  }' > build/release/release.json

notes=$(<scripts/release-notes.md.in)
notes=${notes//@RELEASE@/$release}
notes=${notes//@SOURCE_COMMIT@/$source_commit}
printf '%s\n' "$notes" > build/release/notes.md
