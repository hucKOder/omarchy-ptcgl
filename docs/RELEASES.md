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

Prerequisites: a recent `gh` supporting the verification flags below, `curl`,
`jq`, `sha256sum`, and the separately provisioned
[`osslsigncode`](AUR-DEPENDENCIES.md). Obtain these tools through your usual trusted
package/tool workflow before starting. Using a downloaded bundle avoids requiring
a GitHub login for verification; network access may still be needed for Sigstore
trust metadata.

Replace the two selection values below with the release and source commit you
intend to trust. The placeholder values deliberately fail validation.

Run the following block in Bash. It downloads files without executing them,
verifies `install.sh` itself, and only then runs it:

```bash
(
  set -euo pipefail
  release='vX.Y.Z'
  source_commit='FULL_40_HEX_COMMIT'
  [[ $release =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]
  [[ $source_commit =~ ^[0-9a-f]{40}$ ]]
  repository='hucKOder/omarchy-ptcgl'
  work_dir=$(mktemp -d)
  trap 'rm -rf -- "$work_dir"' EXIT
  base="https://github.com/$repository/releases/download/$release"
  for asset in install.sh release.sigstore.json; do
    curl --proto '=https' --proto-redir '=https' -fsSL \
      "$base/$asset" --output "$work_dir/$asset"
  done
  gh attestation verify "$work_dir/install.sh" \
    --bundle "$work_dir/release.sigstore.json" --hostname github.com \
    --repo "$repository" \
    --cert-identity "https://github.com/$repository/.github/workflows/release.yml@refs/tags/$release" \
    --cert-oidc-issuer https://token.actions.githubusercontent.com \
    --source-ref "refs/tags/$release" --source-digest "$source_commit" \
    --signer-digest "$source_commit" --deny-self-hosted-runners \
    --predicate-type https://slsa.dev/provenance/v1
  bash "$work_dir/install.sh" --release "$release" --source-commit "$source_commit"
)
```

For a launcher-only installation or upgrade, append `--launcher-only` to the final
`bash` command. Rerun this procedure with a freshly verified installer for each
selected release. Do not replace it with execution of `main/install.sh` through a
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
   `scripts/release-assets.sh` packages it with the installer and manifest. The
   privileged publish job downloads these assets without executing project code,
   validates their hashes, generates attestations, and verifies them with the same
   identity policy before publishing.
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
