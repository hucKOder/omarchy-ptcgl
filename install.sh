#!/bin/bash

set -euo pipefail

REPOSITORY="hucKOder/omarchy-ptcgl"
RELEASE_WORKFLOW="$REPOSITORY/.github/workflows/release.yml"
launcher_only=false
work_dir=""

usage() {
  printf '%s\n' \
    "Install Pokémon TCG Live for Omarchy" \
    "" \
    "Usage: bash install.sh --release <vX.Y.Z> --source-commit <40-hex> [--launcher-only]" \
    "       bash install.sh --local [--launcher-only]" \
    "" \
    "Installs repository dependencies, desktop integration, and the game for your user." \
    "Requires osslsigncode to be installed separately from reviewed packaging." \
    "--launcher-only  Leave game setup for the first app-menu launch." \
    "Run a downloaded installer only after verifying its release attestation." \
    "--local  Build the explicitly selected local checkout instead of a release."
}

fail() {
  printf '\nError: %s\n' "$*" >&2
  exit 1
}

cleanup() {
  if [[ -n $work_dir ]]; then
    rm -rf -- "$work_dir"
  fi
}

trap cleanup EXIT

main() {
  local hypr_config script_dir release_url launcher expected actual tool asset
  local source_dir="" release="" source_commit="" local_build=false

  while (( $# )); do
    case "$1" in
      --release)
        (( $# >= 2 )) || fail "--release requires a version."
        [[ -z $release ]] || fail "Specify --release only once."
        release=$2
        shift
        ;;
      --source-commit)
        (( $# >= 2 )) || fail "--source-commit requires a full Git commit."
        [[ -z $source_commit ]] || fail "Specify --source-commit only once."
        source_commit=$2
        shift
        ;;
      --local) local_build=true ;;
      --launcher-only) launcher_only=true ;;
      --help | -h) usage; exit 0 ;;
      *) usage >&2; fail "Unknown option: $1" ;;
    esac
    shift
  done

  if [[ $local_build == true ]]; then
    [[ -z $release && -z $source_commit ]] || fail "--local cannot be combined with release selection."
    [[ -n ${BASH_SOURCE[0]:-} ]] || fail "--local requires an installer file in a checkout."
    script_dir=$(dirname -- "$(realpath -- "${BASH_SOURCE[0]}")")
    [[ -f $script_dir/go.mod && -f $script_dir/cmd/ptcgl-omarchy/main.go ]] || fail "--local requires a PTCGL source checkout."
    source_dir=$script_dir
  else
    [[ $release =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || fail "Select an explicit release with --release vX.Y.Z."
    [[ $source_commit =~ ^[0-9a-f]{40}$ ]] || fail "--source-commit must be the release's full 40-character lowercase Git commit."
  fi

  (( EUID != 0 )) || fail "Run as your desktop user, without sudo. Omarchy will request sudo when needed."
  [[ $(uname -s) == Linux && $(uname -m) == x86_64 ]] || fail "This installer requires x86-64 Linux."
  command -v omarchy >/dev/null || fail "This installer requires Omarchy."
  [[ -z ${XDG_CONFIG_HOME:-} || $XDG_CONFIG_HOME == /* ]] || fail "XDG_CONFIG_HOME must be an absolute path."
  hypr_config="${XDG_CONFIG_HOME:-$HOME/.config}/hypr/hyprland.lua"
  [[ -f $hypr_config ]] || fail "Omarchy's Lua Hyprland config is required at $hypr_config (Hyprland 0.55+)."
  command -v osslsigncode >/dev/null || fail "osslsigncode is required to verify the official game installer. Provision it separately from reviewed packaging, then rerun this installer. See https://github.com/$REPOSITORY/blob/main/docs/AUR-DEPENDENCIES.md"

  if [[ -z $source_dir ]]; then
    for tool in curl jq gh sha256sum; do
      command -v "$tool" >/dev/null || fail "$tool is required to verify a release; provision it before running this installer."
    done
    work_dir=$(mktemp -d "${TMPDIR:-/tmp}/ptcgl-omarchy-install.XXXXXXXX")
    release_url="https://github.com/$REPOSITORY/releases/download/$release"
    printf '\nAuthenticating launcher release %s...\n' "$release"
    for asset in release.json release.sigstore.json; do
      curl --proto '=https' --proto-redir '=https' --fail --silent --show-error --location --retry 3 \
        "$release_url/$asset" --output "$work_dir/$asset" || fail "Could not download authenticated release metadata for $release. No source-build fallback is performed."
    done
    gh attestation verify "$work_dir/release.json" \
      --bundle "$work_dir/release.sigstore.json" --hostname github.com \
      --repo "$REPOSITORY" \
      --cert-identity "https://github.com/$RELEASE_WORKFLOW@refs/tags/$release" \
      --cert-oidc-issuer https://token.actions.githubusercontent.com \
      --source-ref "refs/tags/$release" --source-digest "$source_commit" \
      --signer-digest "$source_commit" --deny-self-hosted-runners \
      --predicate-type https://slsa.dev/provenance/v1 || fail "Release attestation verification failed; installation stopped."
    expected=$(jq -er --arg repo "$REPOSITORY" --arg release "$release" --arg commit "$source_commit" '
      select(.schema == 1 and .repository == $repo and .release == $release and
        .source_commit == $commit and .platform == "linux-amd64" and
        .launcher == "ptcgl-omarchy-linux-amd64" and
        (.installer_sha256 | type == "string" and test("^[0-9a-f]{64}$")) and
        (.launcher_sha256 | type == "string" and test("^[0-9a-f]{64}$"))) |
      .launcher_sha256
    ' "$work_dir/release.json") || fail "Release manifest does not match the selected release, source commit, or platform."
    launcher="$work_dir/ptcgl-omarchy-linux-amd64"
    curl --proto '=https' --proto-redir '=https' --fail --silent --show-error --location --retry 3 \
      "$release_url/ptcgl-omarchy-linux-amd64" --output "$launcher" || fail "Could not download the selected launcher."
    actual=$(sha256sum "$launcher")
    [[ ${actual%% *} == "$expected" ]] || fail "Launcher checksum does not match the authenticated manifest."
    chmod 755 "$launcher"
  fi

  printf '\nInstalling Pokémon TCG Live for Omarchy...\n\n'
  omarchy pkg add umu-launcher zenity desktop-file-utils xdg-utils
  omarchy install gaming gpu-lib32

  if [[ -n $source_dir ]]; then
    printf '\nBuilding the launcher...\n'
    omarchy pkg add make go
    make -C "$source_dir" build
    launcher="$source_dir/build/ptcgl-omarchy"
  fi

  printf '\nInstalling the launcher and desktop integration...\n'
  "$launcher" install

  if [[ $launcher_only == false ]]; then
    printf '\nPreparing Proton and the official Pokémon client (this can take a while)...\n'
    "$HOME/.local/bin/ptcgl-omarchy" setup || fail "Game setup was interrupted. Resume with: \"$HOME/.local/bin/ptcgl-omarchy\" setup"
  fi

  printf '\nInstalled! Open Pokémon TCG Live (Omarchy) from the app launcher.\n'
  if [[ $launcher_only == true ]]; then
    printf 'The first launch will prepare Proton and install the official game.\n'
  fi
  printf 'Terminal launch: %q\n' "$HOME/.local/bin/ptcgl-omarchy"
}

# Define the whole flow before invoking any subprocesses.
main "$@"
