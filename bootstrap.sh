#!/bin/bash

set -euo pipefail

# This small entry point is trusted from the project's HTTPS source URL. Release
# installers and binaries are authenticated separately before they are executed.
main() {
  local repository="hucKOder/omarchy-ptcgl" release="" source_commit="" launcher_only=false
  local arg tool option verify_help base object_type major minor rest
  local -a install_args=()
  fail() { printf '\nError: %s\n' "$*" >&2; exit 1; }
  while (( $# )); do
    case "$1" in
      --release|--source-commit)
        (( $# >= 2 )) || fail "$1 requires a value."
        arg=$1
        if [[ $arg == --release ]]; then release=$2; else source_commit=$2; fi
        shift
        ;;
      --launcher-only) launcher_only=true ;;
      --help|-h)
        printf '%s\n' 'Install Pokémon TCG Live for Omarchy' \
          'Usage: bash bootstrap.sh [--release vX.Y.Z] [--source-commit <40-hex>] [--launcher-only]'
        return
        ;;
      *) fail "Unknown option: $1" ;;
    esac
    shift
  done
  [[ -z $release || $release =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || fail 'Invalid release version.'
  [[ -z $source_commit || ( -n $release && $source_commit =~ ^[0-9a-f]{40}$ ) ]] || fail '--source-commit requires an explicit release and a full lowercase Git commit.'
  (( EUID != 0 )) || fail 'Run as your desktop user, without sudo.'
  [[ $(uname -s) == Linux && $(uname -m) == x86_64 ]] || fail 'x86-64 Linux is required.'
  command -v omarchy >/dev/null || fail 'Run this on Omarchy.'
  [[ -z ${XDG_CONFIG_HOME:-} || $XDG_CONFIG_HOME == /* ]] || fail 'XDG_CONFIG_HOME must be an absolute path.'
  [[ -f ${XDG_CONFIG_HOME:-$HOME/.config}/hypr/hyprland.lua ]] || fail 'Omarchy Lua Hyprland configuration is required (Hyprland 0.55+).'

  printf '\nPreparing installation tools from the system repositories...\n'
  omarchy pkg add github-cli curl jq coreutils ca-certificates
  for tool in curl jq gh sha256sum; do
    command -v "$tool" >/dev/null || fail "Missing $tool after installing bootstrap tools."
  done
  verify_help=$(gh attestation verify --help) || fail 'Update github-cli to a version supporting artifact attestations.'
  for option in --bundle --hostname --repo --cert-identity --cert-oidc-issuer --source-ref --source-digest --signer-digest --deny-self-hosted-runners --predicate-type; do
    [[ $verify_help == *"$option"* ]] || fail "GitHub CLI is missing $option. Update github-cli through Omarchy, then retry."
  done

  work_dir=$(mktemp -d "${TMPDIR:-/tmp}/ptcgl-omarchy-bootstrap.XXXXXXXX")
  trap 'rm -rf -- "$work_dir"' EXIT
  fetch() {
    curl --proto '=https' --proto-redir '=https' --fail --silent --show-error --location --retry 3 \
      "$1" --output "$2" || fail 'Download failed. Retry the installation command.'
  }
  if [[ -z $release ]]; then
    fetch "https://api.github.com/repos/$repository/releases/latest" "$work_dir/selection.json"
    release=$(jq -er '.tag_name | strings' "$work_dir/selection.json") || fail 'Could not select the latest release.'
  fi
  [[ $release =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || fail 'GitHub returned an invalid release version.'
  IFS=. read -r major minor rest <<< "${release#v}"
  (( 10#$major > 0 || 10#$minor >= 4 )) || fail 'One-command installation needs v0.4.0 or later, with built-in MSI verification. Until it is published, use the existing release instructions.'
  if [[ -z $source_commit ]]; then
    # Use GitHub's tag metadata as the selection authority, never an unverified
    # release manifest. Explicit version/commit arguments bypass this discovery.
    fetch "https://api.github.com/repos/$repository/git/ref/tags/$release" "$work_dir/selection.json"
    for rest in 1 2 3 4 5; do
      source_commit=$(jq -er '.object.sha | strings | select(test("^[0-9a-f]{40}$"))' "$work_dir/selection.json") || fail 'Invalid source commit in GitHub tag metadata.'
      object_type=$(jq -er '.object.type | strings' "$work_dir/selection.json") || fail 'Invalid GitHub tag metadata.'
      [[ $object_type != commit ]] || break
      [[ $object_type == tag ]] || fail 'The selected tag does not identify a source commit.'
      fetch "https://api.github.com/repos/$repository/git/tags/$source_commit" "$work_dir/selection.json"
    done
    [[ $object_type == commit ]] || fail 'Too many nested annotated tags.'
  fi
  printf '\nInstalling %s\nSource commit: %s\n' "$release" "$source_commit"
  base="https://github.com/$repository/releases/download/$release"
  fetch "$base/install.sh" "$work_dir/install.sh"
  fetch "$base/release.sigstore.json" "$work_dir/release.sigstore.json"
  gh attestation verify "$work_dir/install.sh" \
    --bundle "$work_dir/release.sigstore.json" --hostname github.com \
    --repo "$repository" \
    --cert-identity "https://github.com/$repository/.github/workflows/release.yml@refs/tags/$release" \
    --cert-oidc-issuer https://token.actions.githubusercontent.com \
    --source-ref "refs/tags/$release" --source-digest "$source_commit" \
    --signer-digest "$source_commit" --deny-self-hosted-runners \
    --predicate-type https://slsa.dev/provenance/v1 || fail 'Installer authentication failed; the launcher and game have not been changed.'
  [[ $launcher_only == false ]] || install_args+=(--launcher-only)
  bash "$work_dir/install.sh" --release "$release" --source-commit "$source_commit" "${install_args[@]}"
}

# Read the entire script before commands that might prompt on the terminal.
main "$@"
