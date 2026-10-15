# Simpler installation

Installation simplification is being delivered in stages. The v0.3.0 release adds
complete release-specific instructions, prerequisite checks, and desktop QoL. The
independent onboarding review on 2026-10-15 also identified dependency provisioning
and recovery work that remains to be completed below.

## Target experience

Choose a supported release, run the documented installation, open the app, and
sign in. A normal user should not need to build packages, inspect a PKGBUILD,
clone this repository, or transcribe a source commit.

Keep release selection explicit. Show the selected version and source commit in
the published instructions and installation output. Generate those values from
the release workflow's tag and checked-out source, rather than discovering the
expected commit from an unverified downloaded manifest.

## Delivery order

| Priority | Work | Dependency |
| --- | --- | --- |
| P0 | Resolve verifier distribution | Needs a supported package channel and an accountable maintainer |
| P1 | Publish one complete, release-specific installation procedure | Can start immediately alongside P0 |
| P1 | Improve prerequisite checks and failure messages | Use the selected provisioning route once P0 is resolved |
| P1 | Make setup progress and recovery predictable | Repository-local launcher changes |
| P1 | Validate the complete new-user journey | Requires the above work and published release artifacts |

The project maintainer owns the repository changes and release validation.
Package-channel acceptance requires cooperation with that channel's maintainers.
Remaining launcher QoL work follows this installation work; download progress is
part of the setup improvements below.

## 1. Resolve verifier distribution

This is the critical path. Shorter documentation alone cannot remove the current
requirement for each player to review and build `osslsigncode`.

- [x] Investigate existing Arch/Omarchy package channels. The
  [findings](AUR-DEPENDENCIES.md#binary-package-route-pending) record the AUR
  version/maintainer and absence from the official Arch search, Omarchy recipes,
  and Omarchy's published x86-64 stable/rc/edge databases. A supported binary
  route remains pending a package contribution.
- [ ] If there is no suitable package, establish whether an existing distributor
  will accept one. Record the concrete blocker if that route is unavailable;
  creating a project-owned repository requires a separate maintenance decision.
- [ ] Validate the chosen package on clean Omarchy against the official MSI's
  signature, certificate chain, and publisher checks.
- [ ] Update [the dependency guide](AUR-DEPENDENCIES.md) with the supported user
  route. Keep packaging review/build instructions for maintainers and advanced
  source installations.

**Done when:** a new player can obtain the verifier through a documented,
supported package transaction without personally auditing or building packaging.
Until then, report the provisioning gap explicitly and describe the install path
as requiring separately prepared dependencies.

## 2. Publish one complete installation procedure

- [x] Generate release installation instructions in the read-only build job,
  using the release tag and full checked-out commit. The publish job consumes
  the generated instructions as data for the release notes.
- [x] Include an executable Bash block with the actual version and commit already
  filled in. It must authenticate the downloaded installer before executing it.
- [x] Make the README's main install path point directly to the supported release
  instructions. Clearly state that the block performs the installation; remove
  the apparent second `bash ./install.sh` step.
- [x] Put prerequisites, expected prompts, expected completion, and the next
  launch action together. Keep source-build and maintainer procedures in their
  own sections.
- [x] Describe `update --check` as optional; `update` already skips an unchanged
  installer. Separate normal game installation from launcher-only upgrades.
- [x] Distinguish published features from unreleased changes and replace
  inaccurate "one-command" claims. Supply a ready-to-run v0.2.0 fallback using
  the source commit confirmed against its release announcement and annotated tag.
- [ ] Correct obsolete installation guidance in older published release notes
  without replacing their tagged source or artifacts.

Likely files: `README.md`, `docs/RELEASES.md`, `docs/UPDATES.md`,
`scripts/release-assets.sh`, `.github/workflows/release.yml`.

**Done when:** the release's documented block works without editing placeholders
or consulting maintainer documentation. It agrees with the release manifest and
announced source commit. Tests exercise the generated procedure and reject
mismatched or tampered installer bytes before execution.

Implementation: `scripts/release-notes.md.in` is rendered by
`scripts/release-assets.sh` and consumed by the publish job. Tests execute the
generated block with isolated tools and ensure the v0.2.0 documentation matches
the same generated procedure. Published-release smoke testing remains in step 5.

## 3. Explain prerequisites before starting

- [x] Document tested GitHub CLI 2.102.0 capabilities and the repository-tool
  provisioning command for `gh`, `curl`, `jq`, and `sha256sum`. Accept other
  versions with the required flags; the verifier route remains pending step 1.
- [x] Add checks to the generated bootstrap procedure and installer for missing
  tools, unsupported `gh` verification flags, invalid release selections, and an
  unsupported host. Each failure should name the problem and the next action.
- [ ] Report the selected release and current installation stage. On failure,
  explain whether a usable launcher was installed and how to continue.

Likely files: the generated bootstrap instructions, `install.sh`,
`internal/installer/install_test.go`.

**Done when:** missing prerequisites and unsupported verifier capabilities fail
before installation changes. Invalid selections produce an explanation rather
than an unexplained shell exit. Authentication failures preserve the installed
launcher and never fall back to building `main`.

## 4. Make progress and recovery predictable

- [ ] Show downloading, verifying, extracting, and installing as distinct phases.
  Show downloaded MiB and percentage when the server provides a total; use
  indeterminate progress otherwise. Reuse the existing download-progress QoL item.
- [ ] Allow download cancellation with temporary-file cleanup and a clear retry
  action. Make cancellation availability follow the current phase.
- [ ] Give one recovery action for each failed stage:
  - Before the launcher is installed: repeat the verified release procedure.
  - Interrupted download or runtime setup: resume with `ptcgl-omarchy setup`.
  - Failed or interrupted MSI application: use `ptcgl-omarchy repair`.
- [ ] Make those recovery actions reliable. Re-stage an incomplete managed
  runtime instead of requiring the user to move directories manually. Keep
  custom runtime overrides user-managed.
- [ ] Use successful installation state as well as executable presence to detect
  incomplete MSI setup. An executable left by a failed install must not cause
  first launch to incorrectly declare installation complete.
- [ ] Align CLI errors, graphical reports, diagnostics, and documentation with
  the same recovery guidance.

Likely files: `internal/app/download.go`, `internal/app/proton.go`,
`internal/app/setup.go`, `internal/app/ui.go`, `internal/app/doctor.go`,
`docs/UPDATES.md`, and relevant existing tests.

**Done when:** isolated tests cover known/unknown-length downloads, cancellation,
incomplete managed runtimes, and an interrupted MSI that leaves an executable.
Retry preserves the prefix, settings, useful completed downloads, and the
previous successful installation receipt until replacement succeeds.

## 5. Validate from a clean user's perspective

- [x] Review and smoke-test the candidate on the existing Omarchy installation.
  Independent review findings were fixed; real desktop actions, same-MSI
  check/update, repair, post-repair startup, and window operations passed on
  2026-10-15. See [the validation record](VALIDATION.md) for evidence and scope.
- [x] Test fresh application data on the existing host: new HOME/XDG paths,
  downloaded runtime, initialized prefix, authenticated MSI installation,
  idempotent setup, and first browser sign-in. An explicit temporary callback
  route was required to keep browser delivery in the isolated test prefix; the
  normal handler and installation were restored and checksum-verified afterward.
- [ ] Exercise the documented path on clean supported Omarchy with no launcher,
  game prefix, build toolchain, or pre-provisioned verifier. Use a separate test
  machine or disposable environment; complete graphics/browser/gameplay checks
  on a suitable real desktop.
- [ ] Repeat using the published release assets and instructions. A source-tree
  or stubbed verification test alone does not establish release readiness.
- [ ] Check the full path, launcher-only upgrade, and recovery after interrupted
  downloads and MSI application. Include missing/old prerequisites and failed
  authentication as negative cases.
- [ ] Have a first-time tester follow only the user-facing instructions, without
  developer coaching. Record manual decisions/actions, time to launch, and any
  points where they leave the normal installation guide.
- [ ] Record the exact release, source commit, host/tool versions, and results in
  [the validation record](VALIDATION.md).

**Done when:** the tester reaches browser sign-in and gameplay through the
documented route. The target is about four coarse user tasks: select the
supported release, perform/authorize installation, launch, and sign in. Count
user-owned tasks rather than commands inside an automated block; record download
waiting separately. Package building or manual commit transcription means the
simple-install target has not been met.

## Verification and distribution boundaries

Preserve installer authentication before execution, authenticated release
metadata, launcher digest checks, the pinned Proton checksum, and MSI
signature/certificate/publisher verification. The current installer and metadata
checks protect different boundaries; removing one is not a documentation fix.

Reusing the downloaded attestation bundle may save a request later. A new
bootstrap verification protocol is outside this first simplification pass.

A native signed launcher package is a follow-up once the dependency route is
settled. It would require package-owned executable paths and per-user desktop
integration. Document any distributor trust handoff explicitly; do not silently
replace the current release-authentication policy with package signatures.

## Next implementation slice

Follow up on the Omarchy package contribution and implement setup progress and
recovery. Release-specific instruction generation and the channel investigation
are implemented; the distribution handoff and published-release acceptance are
still open. Do not call the clean-machine installation problem solved until both
pass.
