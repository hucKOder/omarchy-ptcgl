# Authenticated launcher releases

## Trust model

Release artifacts use GitHub artifact attestations backed by short-lived Sigstore
certificates. The trusted release identity is:

- Repository: `hucKOder/omarchy-ptcgl` on `github.com`.
- Workflow: `.github/workflows/release.yml` at the selected `refs/tags/vX.Y.Z`.
- Source and signer digest: the explicitly selected full 40-character Git commit.
- OIDC issuer: `https://token.actions.githubusercontent.com`.
- Runner: GitHub-hosted, with SLSA v1 provenance.

The verifier is the independently provisioned GitHub CLI, using its trusted
Sigstore roots. No project signing private key is stored in this repository or
required as an Actions secret. Certificate/trust-root rotation is handled by
GitHub, Sigstore, and the GitHub CLI's supported trust mechanism.

The attestation authenticates artifact bytes and the producing workflow identity;
it does not prove the source is harmless or prevent compromise of an authorized
release workflow. Choose and review the source commit. Keep its value in a trusted
record, or obtain it from a trusted maintainer announcement. A commit copied only
from an unverified downloaded manifest is not an independently established pin.

## Verified release installation

For releases built from this checkout, use the complete **Install** block in the
[selected release's notes](https://github.com/hucKOder/omarchy-ptcgl/releases).
The workflow fills in its version and source commit; no manual substitution is
needed. The block performs the full installation, including verification before
the downloaded installer runs.

Prepare bootstrap tools with `omarchy pkg add github-cli curl jq coreutils` and
provision [`osslsigncode`](AUR-DEPENDENCIES.md) separately. GitHub CLI 2.102.0 is
tested; required verification capabilities are checked before downloading.
Bundle-based verification does not require a GitHub login. Network access may
still be needed for Sigstore trust metadata.

### Existing v0.2.0 release

The following block is ready to run for **v0.2.0**, whose
[published announcement](https://github.com/hucKOder/omarchy-ptcgl/releases/tag/v0.2.0)
and annotated tag identify source commit
`5e3e96bff7a5bcff0b093dc64dc12538d813148e`. This provides complete instructions
for that release without changing its existing artifacts. For another release,
use that release's own installation block.

Copy the entire block into Bash as your regular desktop user. It downloads the
installer as data, verifies `install.sh`, and then runs the installation:

```bash
(
  set -euo pipefail
  release='v0.2.0'
  source_commit='5e3e96bff7a5bcff0b093dc64dc12538d813148e'
  repository='hucKOder/omarchy-ptcgl'
  fail() { printf 'Error: %s\n' "$*" >&2; exit 1; }
  (( EUID != 0 )) || fail 'Run as your desktop user, without sudo.'
  [[ $(uname -s) == Linux && $(uname -m) == x86_64 ]] || fail 'x86-64 Linux is required.'
  command -v omarchy >/dev/null || fail 'Run this on Omarchy.'
  [[ -z ${XDG_CONFIG_HOME:-} || $XDG_CONFIG_HOME == /* ]] || fail 'XDG_CONFIG_HOME must be an absolute path.'
  [[ -f ${XDG_CONFIG_HOME:-$HOME/.config}/hypr/hyprland.lua ]] || fail 'Omarchy Lua Hyprland configuration is required (Hyprland 0.55+).'
  for tool in curl jq gh sha256sum; do
    command -v "$tool" >/dev/null || fail "Missing $tool. Run: omarchy pkg add github-cli curl jq coreutils"
  done
  command -v osslsigncode >/dev/null || fail "osslsigncode is required. See https://github.com/$repository/blob/$source_commit/docs/AUR-DEPENDENCIES.md"
  verify_help=$(gh attestation verify --help) || fail 'Update github-cli to a version supporting artifact attestations.'
  for option in --bundle --hostname --repo --cert-identity --cert-oidc-issuer --source-ref --source-digest --signer-digest --deny-self-hosted-runners --predicate-type; do
    [[ $verify_help == *"$option"* ]] || fail "GitHub CLI is missing $option. Update github-cli through Omarchy, then retry this block."
  done
  work_dir=$(mktemp -d)
  trap 'rm -rf -- "$work_dir"' EXIT
  base="https://github.com/$repository/releases/download/$release"
  printf 'Verifying installer for %s\nSource commit: %s\n' "$release" "$source_commit"
  for asset in install.sh release.sigstore.json; do
    curl --proto '=https' --proto-redir '=https' -fsSL --retry 3 \
      "$base/$asset" --output "$work_dir/$asset" || fail 'Download failed. Retry this block; installation has not started.'
  done
  gh attestation verify "$work_dir/install.sh" \
    --bundle "$work_dir/release.sigstore.json" --hostname github.com \
    --repo "$repository" \
    --cert-identity "https://github.com/$repository/.github/workflows/release.yml@refs/tags/$release" \
    --cert-oidc-issuer https://token.actions.githubusercontent.com \
    --source-ref "refs/tags/$release" --source-digest "$source_commit" \
    --signer-digest "$source_commit" --deny-self-hosted-runners \
    --predicate-type https://slsa.dev/provenance/v1 || fail 'Installer authentication failed; installation has not started.'
  bash "$work_dir/install.sh" --release "$release" --source-commit "$source_commit"
)
```

For a launcher-only installation or upgrade, append `--launcher-only` to the final
`bash` command inside the block. Do not run a second installer command afterward.
Rerun the selected release's complete procedure for each installation or upgrade.
Do not replace it with execution of `main/install.sh` through a
pipe: an installer cannot establish trust in its own code after it starts running.

The installer authenticates `release.json` from the same release identity, checks
its version/source/platform, and checks the downloaded launcher's SHA-256 before
installing repository dependencies or executing the launcher. Authentication and
download failures preserve the existing installed launcher. Normal setup failures
after a successfully authenticated launcher installation remain resumable through
the installed launcher's `setup` command.

Releases published before this workflow, including the original `v0.1.0`, do not
have these assets. They fail the authenticated installation procedure. An explicit
reviewed local checkout can be built with `bash ./install.sh --local`.

## Release assets

Each release contains:

- `install.sh` — the version-specific installer.
- `ptcgl-omarchy-linux-amd64` — the launcher.
- `release.json` — schema/version/repository/source/platform and both SHA-256 hashes.
- `release.sigstore.json` — a signed attestation bundle covering the other assets.

The manifest is JSON data, never sourced as shell code. Filenames and download
locations are fixed by the installer rather than supplied by that manifest.

## Maintainer release procedure

1. Review and commit the release source, including installer, tests, and workflow.
   Run `make check`, `make test`, and `make build`.
2. Select a new version and record its source commit. Create and push its tag when
   ready to publish. The tag must identify the reviewed commit; protect release
   tags and workflow changes using repository access controls.
3. The read-only build job tests the source and builds the versioned launcher.
   `scripts/release-assets.sh` packages it with the installer, manifest, and
   complete release-specific installation notes. The
   privileged publish job downloads these assets without executing project code,
   validates their hashes, generates attestations, and verifies them with the same
   identity policy before publishing. It uses the generated notes as data and
   does not execute project code to produce release instructions.
4. Check the published assets and exercise the verified download/attestation steps
   against the new release before announcing it. Record the full source commit
   alongside the version in the announcement.

The release workflow requires `contents: write`, `id-token: write`, and
`attestations: write` only in its publish job. Public repositories support this
without a custom signing secret. Actions are pinned to commit IDs; review both
their upstream changes and the pinned IDs when updating them.

Existing releases are not overwritten by the workflow. Recover from an invalid or
compromised release by announcing the affected version/commit and publishing a new
reviewed version. An old valid attestation is not automatically revoked: users
must avoid explicitly selecting the affected commit. Keep a previously verified
installer/release selection for recovery; earlier binaries can be reinstalled
through the same verification procedure.

## Verification coverage

Local tests exercise installer authentication policy, manifest parsing, artifact
tampering, failures, and preservation of the existing launcher with stubbed
network/attestation commands. They test orchestration rather than reimplementing
Sigstore cryptography. The publish job additionally verifies real generated
attestations using `gh` before creating a release. A real new-release smoke test
requires a GitHub Actions release run.
