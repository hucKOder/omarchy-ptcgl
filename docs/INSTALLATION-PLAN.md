# Simpler installation

Installation simplification is being delivered in stages. The v0.3.0 release adds
complete release-specific instructions, prerequisite checks, and desktop QoL. The
independent onboarding review on 2026-10-15 also identified dependency provisioning
and recovery work. The v0.4.0 release replaces the external verifier with built-in
Relic verification and adds a one-command source bootstrap. Published artifacts
and isolated launcher-only installation are verified; clean-machine acceptance
remains pending.

The [2026-10-22 security review](SECURITY-AUDIT.md) adds a prerequisite security
slice: patched Go certificate trust, complete MSI chain revocation, and
release-source protections precede new release acceptance. The source bootstrap
also depends on the separately provisioned `curl`/`gh` trust paths. See the
[main security priorities](PLAN.md#security-priority--2026-10-22).

The bulk source patch now requires Go 1.27.3, validates complete certificate
revocation paths and archive links, filters runtime credentials, and adds release
policy/rebuild gates. Read-only candidate checks passed against the official MSI
and both public compromised-CA test chains. GitHub protection activation and
closed-game installation/gameplay acceptance remain prerequisites to publication.

## Target experience

Choose a supported release, run the documented installation, open the app, and
sign in. A normal user should not need to build packages, inspect a PKGBUILD,
clone this repository, or transcribe a source commit.

Show the selected version and source commit in
the published instructions and installation output. Generate those values from
the release workflow's tag and checked-out source, rather than discovering the
expected commit from an unverified downloaded manifest. The one-command bootstrap
uses GitHub's HTTPS tag API for automatic selection and displays that selection;
independent version/commit pins remain available through explicit arguments or
the release-specific manual block. See [the trust model](RELEASES.md).

## Delivery order

| Priority | Work | Dependency |
| --- | --- | --- |
| P0 | Patch Go trust handling and complete publisher/timestamp chain revocation | SEC-01/SEC-02 in the security audit; required before a new launcher release |
| P1, before release | Protect release source/tags and independently validate published bytes | SEC-04; preserve the existing attestation and source-selection policy |
| P0 | Remove external verifier distribution | Released in v0.4.0 with pinned Relic; published-artifact verification passed |
| P1 | Publish one complete, release-specific installation procedure | Can start immediately alongside P0 |
| P1 | Improve prerequisite checks and failure messages | Use the selected provisioning route once P0 is resolved |
| P1 | Make setup progress and recovery predictable | Repository-local launcher changes |
| P1 | Validate the complete new-user journey | Requires the above work and published release artifacts |

The project maintainer owns the repository changes and release validation.
Package-channel acceptance requires cooperation with that channel's maintainers.
Remaining launcher QoL work follows this installation work; download progress is
part of the setup improvements below.

## 1. Resolve verifier distribution

**Superseded for v0.4.0:** built-in Relic verification removes the external package
dependency. The original package investigation below remains relevant to legacy
v0.2.0/v0.3.0 installations. No upstream package contribution is required for the
new flow. The replacement preserves content/signature, publisher, code-signing
certificate, and trusted timestamp checks. Dependency pins and negative tests
are maintained with the launcher.

This was the critical path for the legacy flow. Shorter documentation alone cannot remove its
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

**Original package-route acceptance:** a new player can obtain the verifier through a documented,
supported package transaction without personally auditing or building packaging.
The legacy published installation path still requires separately prepared
dependencies. The v0.4.0 source path does not.

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
the same generated procedure. Published v0.3.0 smoke testing is recorded in step 5.

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

Included in v0.5.0 as UX milestone 2, with isolated regressions and existing-installation
desktop smoke testing. The maintainer approved stable promotion after the installed
candidate's successful logout/login and gameplay test. Real MSI/runtime recovery
and clean-OS acceptance remain unverified follow-ups, not completed checks.
See [the milestone status](PLAN.md).

- [x] Show downloading, verifying, extracting, and installing as distinct phases.
  Show downloaded MiB and percentage when the server provides a total; use
  indeterminate progress otherwise. Reuse the existing download-progress QoL item.
- [x] Allow download cancellation with temporary-file cleanup and a clear retry
  action. Make cancellation availability follow the current phase.
- [x] Give one recovery action for each failed stage:
  - Before the launcher is installed: repeat the verified release procedure.
  - Interrupted download or runtime setup: resume with `ptcgl-omarchy setup`.
  - Failed or interrupted MSI application: use `ptcgl-omarchy repair`.
- [x] Make those recovery actions reliable. Re-stage an incomplete managed
  runtime instead of requiring the user to move directories manually. Keep
  custom runtime overrides user-managed.
- [x] Use successful installation state as well as executable presence to detect
  incomplete MSI setup. An executable left by a failed install must not cause
  first launch to incorrectly declare installation complete.
- [x] Align CLI errors, graphical reports, diagnostics, and documentation with
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
- [x] Verify published v0.3.0 assets and execute its generated installation block
  for a launcher-only upgrade. All three attestations and manifest hashes passed;
  the installed binary matched the published artifact, all 16 diagnostics passed,
  and preserved game/configuration state retained its hashes.
- [ ] Complete the published-release full-install and interrupted-download/MSI
  recovery checks. Include missing/old prerequisites and failed authentication as
  negative cases. The launcher-only upgrade above is already verified.
- [ ] Have a first-time tester follow only the user-facing instructions, without
  developer coaching. Record manual decisions/actions, time to launch, and any
  points where they leave the normal installation guide.
- [x] Record v0.3.0, its source commit, host/tool versions, and verified results in
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

The new source bootstrap is initial trusted code fetched over GitHub HTTPS. It
automates tool provisioning and tag selection, then uses the existing release
attestation protocol. This initial trust and the distinction from independently
pinned manual installation are documented in the release guide.

A native signed launcher package is a follow-up once the dependency route is
settled. It would require package-owned executable paths and per-user desktop
integration. Document any distributor trust handoff explicitly; do not silently
replace the current release-authentication policy with package signatures.

## Next implementation slice

Complete the security prerequisites above, then validate the patched verifier and
one-command bootstrap on clean Omarchy before implementing setup progress and
recovery. Use isolated fixtures during the user's active game session; perform
installed-launcher/runtime/browser acceptance after the game closes. The
existing v0.3.0 release-specific installation and launcher-only upgrade remain
verified historical records. Legacy verifier packaging is no longer on the new
installation's critical path.
