# Authenticated launcher releases

## One-command installation (v0.4.0 and later)

The README's one-line command downloads `bootstrap.sh` from this project's source
on GitHub over HTTPS. The bootstrap is the initial trusted code, like the commands
copied from this page; it cannot authenticate itself after execution starts.

It installs `github-cli`, `curl`, `jq`, `coreutils`, and `ca-certificates` through
Omarchy's system-repository tooling. It selects the latest stable release and
resolves its source commit through GitHub's tag API, following annotated tags.
This trusts GitHub's HTTPS metadata for release selection. The selected version
and full commit are displayed before downloading release code. The bootstrap
never takes the expected commit from the release manifest.

The bootstrap then authenticates the release installer using the attestation
policy below **before executing it**. The installer authenticates the manifest
and launcher before installing them. Failed verification never falls back to a
source build. Bootstrap tools may already have been installed when verification
fails; the existing launcher and game are preserved.

The v0.4.0 release is the first supported version. Older published
releases still need their original prerequisites and installation instructions.

To select an independently established version and commit, download/review
`bootstrap.sh` and pass `--release vX.Y.Z --source-commit FULL_40_HEX_COMMIT`,
or use the release-specific complete block below. `--launcher-only` skips game
setup. No GitHub login is required.

MSI verification is built into the launcher using Relic v8.2.0 and Go's X.509
verification, with dependencies pinned by `go.mod`/`go.sum`. CIRCL is explicitly
pinned to v1.6.3 to include fixes absent from Relic's original dependency set.
Verification checks the MSI content digest, signature, code-signing certificate
chain, timestamp signature/chain when present, and the primary signer's exact
Pokémon publisher identity. In the security candidate, every non-root certificate
in an accepted publisher or timestamp chain must advertise a usable CRL. Those
CRLs are checked for issuer signatures, freshness, scope, and revocation;
missing, unavailable, or invalid revocation evidence stops verification. Alternate
X.509 paths are tried until one complete non-revoked chain succeeds. Delta, indirect, and
reason-restricted CRLs are rejected rather than assumed to cover the signer.
System certificate authorities are used; embedded
certificates do not become trusted roots. A private child process bounds
verification with a two-minute deadline. No external verifier or AUR package is
needed. This removes the AUR packaging trust boundary; it is not a claim that
the replacement library is vulnerability-free. Release maintenance includes
dependency vulnerability checks and negative verification tests.

The security candidate raises the build minimum to Go 1.27.3, including the
compromised-public-key distrust fix for GO-2026-6809 / CVE-2026-111943. Previously
published v0.4.0 bytes still contain Go 1.27.2. The patched source must be built
and published as a new version; neither source changes nor an updated compiler
repair an already-installed binary. `curl`, `gh`, and the initial source bootstrap
have separate trust paths that must also be assessed on the installation host.

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

For the manual block, prepare bootstrap tools with
`omarchy pkg add github-cli curl jq coreutils ca-certificates`.
GitHub CLI 2.102.0 is tested; required verification capabilities are checked before downloading.
Bundle-based verification does not require a GitHub login. Network access may
still be needed for Sigstore trust metadata.

### Existing v0.2.0 release

The v0.2.0 and v0.3.0 launchers still need a separately provisioned
[`osslsigncode`](AUR-DEPENDENCIES.md). Their published binaries are unchanged.

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
The source bootstrap described above is a separate initial trust choice. Running
`main/install.sh` through a pipe is not equivalent to either verified procedure.

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

1. Review and commit the release source, including bootstrap, installer, dependency
   pins, tests, and workflow. After dependency changes, run `make licenses` to
   refresh the notices embedded in the binary. Run `make check`, `make test`,
   `make check-shell`, `make security`, and `make build` with the patched toolchain
   selected by `go.mod`. Vulnerability checks also run daily between releases.
2. Select a new version and record its source commit. Create and push its tag when
   ready to publish. The tag must identify the reviewed commit; protect release
   tags and workflow changes using repository access controls.
3. The read-only policy job requires the tag's commit to be in CI-protected `main`
   history and checks the settings below. The read-only build job tests the source
   and builds the versioned launcher.
   `scripts/release-assets.sh` packages it with the installer, manifest, and
   complete release-specific installation notes. A second, cache-free checkout
   rebuilds with the same exact Go version and compares the launcher and installer
   bytes. Its downloaded comparison assets stay under ignored `build/` so they do
   not change the rebuild's VCS dirty stamp. The privileged publish job uses the
   protected `release` environment and downloads these assets without executing project code,
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

### Required GitHub release settings

These repository settings are external to the source patch. Configure them before
publishing; `scripts/check-release-policy.sh` fails closed while they are missing.
It uses read-only ruleset/environment APIs, not a stored administration token.

- An **active branch ruleset** explicitly covering `refs/heads/main` (or all
  branches), with no excluded refs or bypass actors. An unresolved
  `~DEFAULT_BRANCH` alias does not satisfy the check. Require PRs with zero
  mandatory approvals, no mandatory code-owner or last-push approval, stale-approval
  dismissal, resolved review threads, and up-to-date checks named `test` and `vulnerabilities`, each bound
  to the GitHub Actions integration (`integration_id: 15368`). Prevent
  force-pushes and deletion. Classic branch protection alone does not satisfy
  this policy check.
- An **active tag ruleset** covering `refs/tags/v*` (or all tags), with no excluded
  refs or bypass actors, preventing tag updates and deletion. New tags remain
  possible; the workflow separately requires their source in `main` history.
- A **`release` environment** requiring approval by `@hucKOder`, permitting
  self-review, disabling administrator bypass, and allowing only tags matching
  `v*` through custom deployment restrictions.
- `.github/CODEOWNERS` records ownership and routes optional reviews. The project
  currently has one maintainer; requiring a second person's PR or release
  approval would make its release process unusable. The enforced policy therefore
  requires CI-tested PRs and explicit approval by the maintainer, not independent
  human review. These controls do not prevent a compromised sole-maintainer account
  from approving malicious source or releases. If another maintainer joins, revise
  the policy, settings, fixtures, and documentation together.

The policy gate validates these concrete settings, not every possible equivalent
GitHub configuration. Its fixtures exercise accepted settings and rejection of
missing PR/CI, unbound or wrong-source checks, bypass actors, mutable/excluded tags,
unusable solo-maintainer approval settings, unexpected reviewers,
administrator release bypass, unexpected
repository/ref, API failures, and branch-enabled deployments.

The second rebuild is an independent runner execution within the same repository
trust domain. It detects an artifact/source mismatch under the reviewed workflow;
an account capable of rewriting the workflow or protections remains a threat.
Keep independently established source pins and preserve GitHub audit/release logs.

Existing releases are not overwritten by the workflow. Recover from an invalid or
compromised release by announcing the affected version/commit and publishing a new
reviewed version. An old valid attestation is not automatically revoked: users
must avoid explicitly selecting the affected commit. Keep a previously verified
installer/release selection for recovery; earlier binaries can be reinstalled
through the same verification procedure.

For an incident, pause publication, preserve workflow/artifact/provenance evidence,
identify the affected versions, commits, and digests, and publish an explicit
advisory. Revoke exposed maintainer credentials from a clean environment, review
protection/workflow changes, and rebuild a new reviewed version. A valid signature
or attestation on malicious bytes remains valid cryptographically; withdrawing a
release or issuing an advisory does not automatically revoke every cached copy.
Users need explicit affected-version guidance and a verified recovery release.

## Verification coverage

Local tests exercise installer authentication policy, manifest parsing, artifact
tampering, failures, and preservation of the existing launcher with stubbed
network/attestation commands. They test orchestration rather than reimplementing
Sigstore cryptography. The publish job additionally verifies real generated
attestations using `gh` before creating a release. A real new-release smoke test
requires a GitHub Actions release run.
