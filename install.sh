#!/bin/bash

set -euo pipefail

REPOSITORY="hucKOder/omarchy-ptcgl"
launcher_only=false
work_dir=""

usage() {
  printf '%s\n' \
    "Install Pokémon TCG Live for Omarchy" \
    "" \
    "Usage: bash install.sh [--launcher-only]" \
    "" \
    "Installs dependencies, desktop integration, and the game for your user." \
    "--launcher-only  Leave game setup for the first app-menu launch." \
    "Run from a checkout to build that checkout instead of downloading a release."
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
  local arg hypr_config script_dir status tag release_url launcher expected actual
  local source_dir=""

  for arg in "$@"; do
    case "$arg" in
      --launcher-only) launcher_only=true ;;
      --help | -h) usage; exit 0 ;;
      *) usage >&2; fail "Unknown option: $arg" ;;
    esac
  done

  (( EUID != 0 )) || fail "Run as your desktop user, without sudo. Omarchy will request sudo when needed."
  [[ $(uname -s) == Linux && $(uname -m) == x86_64 ]] || fail "This installer requires x86-64 Linux."
  command -v omarchy >/dev/null || fail "This installer requires Omarchy."
  [[ -z ${XDG_CONFIG_HOME:-} || $XDG_CONFIG_HOME == /* ]] || fail "XDG_CONFIG_HOME must be an absolute path."
  hypr_config="${XDG_CONFIG_HOME:-$HOME/.config}/hypr/hyprland.lua"
  [[ -f $hypr_config ]] || fail "Omarchy's Lua Hyprland config is required at $hypr_config (Hyprland 0.55+)."

  printf '\nInstalling Pokémon TCG Live for Omarchy...\n\n'
  omarchy pkg add umu-launcher zenity desktop-file-utils xdg-utils
  omarchy pkg aur add osslsigncode
  omarchy install gaming gpu-lib32

  work_dir=$(mktemp -d "${TMPDIR:-/tmp}/ptcgl-omarchy-install.XXXXXXXX")
  # BASH_SOURCE is empty when the installer is piped to Bash.
  if [[ -n ${BASH_SOURCE[0]:-} ]]; then
    script_dir=$(dirname -- "$(realpath -- "${BASH_SOURCE[0]}")")
    if [[ -f $script_dir/go.mod && -f $script_dir/cmd/ptcgl-omarchy/main.go ]]; then
      source_dir=$script_dir
    fi
  fi

  if [[ -z $source_dir ]]; then
    omarchy pkg add curl jq
    printf '\nFinding the latest launcher release...\n'
    status=$(curl --fail --silent --show-error --location --retry 3 \
      --output "$work_dir/release.json" --write-out '%{http_code}' \
      "https://api.github.com/repos/$REPOSITORY/releases/latest") || {
        [[ $status == 404 ]] || fail "Could not check launcher releases. Retry the installer."
      }
    case "$status" in
      200)
        tag=$(jq -er '.tag_name' "$work_dir/release.json")
        [[ $tag =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || fail "Invalid launcher release tag."
        release_url="https://github.com/$REPOSITORY/releases/download/$tag"
        launcher="$work_dir/ptcgl-omarchy-linux-amd64"
        printf '\nDownloading launcher %s...\n' "$tag"
        curl --fail --silent --show-error --location --retry 3 \
          "$release_url/ptcgl-omarchy-linux-amd64" --output "$launcher"
        curl --fail --silent --show-error --location --retry 3 \
          "$release_url/ptcgl-omarchy-linux-amd64.sha256" --output "$launcher.sha256"
        read -r expected _ < "$launcher.sha256" || fail "Invalid launcher checksum."
        [[ $expected =~ ^[0-9a-f]{64}$ ]] || fail "Invalid launcher checksum."
        actual=$(sha256sum "$launcher")
        [[ ${actual%% *} == "$expected" ]] || fail "Launcher checksum mismatch. Retry the installer."
        chmod 755 "$launcher"
        ;;
      404)
        printf '\nNo published release yet; building the launcher from main...\n'
        omarchy pkg add git
        source_dir="$work_dir/source"
        git clone --depth 1 --branch main "https://github.com/$REPOSITORY.git" "$source_dir"
        ;;
      *) fail "Unexpected release response: $status" ;;
    esac
  fi

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

# Define the whole flow before running it, so subprocesses cannot consume the
# remaining installer code when invoked with curl ... | bash.
main "$@"
