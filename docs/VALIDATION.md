# Validation record

Validated on 2026-10-13:

| Component | Version / hardware |
| --- | --- |
| Omarchy | 4.0.4-1 |
| Hyprland | 0.56.2-2, Lua configuration |
| GPU | AMD Radeon RX 9070 family, amdgpu / Mesa |
| Go | 1.27.2 |
| UMU | 1.4.4-1 |
| GE-Proton | GE-Proton11-7-x86_64 |
| osslsigncode | 2.14-2 |

## Completed checks

- Fresh native setup downloaded and checksum-verified the pinned GE-Proton
  archive, extracted it, initialized a separate prefix, and installed the
  official game MSI after Authenticode/publisher verification.
- `update --check` matched the installation receipt; `update` skipped an
  unchanged MSI rather than reinstalling the game.
- Desktop entries passed `desktop-file-validate`.
- The `tpcitcgapp` URI handler selected `ptcgl-omarchy-callback.desktop`.
- Browser authentication callbacks reached the game during startup testing.
- Hyprland reload completed with no configuration errors.
- The live game window had title `Pokemon TCG Live`, class `steam_app_0`,
  `xwayland: true`, `floating: false`, and `fullscreen: 0`.
- The game opened alongside another tiled application on the current workspace.
- A compositor resize changed the game's tile size successfully.
- Repeat launch focused the existing game rather than starting another copy.
- Update checks were rejected while the game session was running.
- `doctor` passed the dependency, runtime, game, callback, desktop-entry,
  and Hyprland configuration checks.
- `make check`, `make test`, and `make build` passed.

The Wine fixed-size hints required `no_max_size` and suppression of client X11
configure requests for reliable tiling. Repeat-launch detection resolves
Proton's `prefix/pfx` alias. Login delivery is detached from the short-lived URI
handler, so a callback subprocess is not terminated by a launcher timeout.

## Manual gameplay acceptance

On 2026-10-15, the user confirmed completing multiple online gameplay sessions
without issues. Online-play acceptance is complete, in addition to the window
startup and integration checks above.

## Changed-installer upgrade acceptance

A real changed-MSI upgrade remains to be recorded when a newer official installer
is available. The automated suite covers changed installer application, same-MSI
no-op updates, explicit repair, preservation of registry/settings data, and failed
updates.

## Local candidate review and smoke test — 2026-10-15

Tested the uncommitted working tree based on
`5e3e96bff7a5bcff0b093dc64dc12538d813148e`, built as
`0.2.0-dev.5e3e96b-dirty`. The installed executable matched the build's SHA-256:

```text
2d3de5115e0d0b81b934a395786c06c54785dd1ca52ed751a570edd39dff57ba
```

Omarchy, Hyprland, Go, UMU, GE-Proton, and osslsigncode versions matched the
2026-10-13 record. GitHub CLI was 2.102.0. The game reported version 1.43.0.

### Independent review and automated checks

An independent agent reviewed the installation/release changes and identified
three issues: RC publication lacked prerelease flags, initialization errors were
invisible from graphical actions, and the standalone update guide omitted the
unreleased-feature distinction. All three were fixed. A read-only follow-up found
no remaining actionable issue in those changes.

- `make check` and `make test` passed after the fixes.
- The development-version build passed.
- Isolated regressions cover actual publication-script argument routing for
  stable/RC/beta tags, graphical initialization errors, terminal-only errors, and
  suppression of callback arguments from diagnostics.

### Real desktop and maintenance checks

- Installed the candidate and refreshed integration again. Both desktop files
  passed `desktop-file-validate`; the browser callback handler was correct.
- Hyprland reload/configuration checks succeeded. Hashes of the main Lua config,
  managed rules, and selected runtime profile were unchanged by installation and
  the integration refresh.
- All 16 `doctor` checks passed, including after repair and game shutdown. The
  actual Diagnostics desktop action displayed the report and dismissed normally.
- The actual **Check installer updates** and **Update game** desktop actions
  downloaded and authenticated the current MSI, displayed the unchanged-installer
  result, and left the installation receipt and registry hash unchanged.
- The **Repair game** desktop action successfully reapplied that MSI in the
  existing prefix and displayed its result. The receipt advanced to `repair` for
  installer SHA-256
  `53bacfb0ecb71e10390bd8cd3cdc2ae75df67329a355e9d1413efb9680416a12`.
- The folder chooser displayed the three locations and cancelled normally.
  Selecting the cache opened the default file manager at the expected cache URI.
  Direct game-folder opening retained spaces/Unicode in the correct URI. The
  absent optional configuration folder produced an explanation without creating
  it.

### Game startup and window behavior

- Launched through the installed desktop entry after repair. The game reached
  its signed-in home screen with the existing account state retained.
- The game window was XWayland, tiled, and initially not fullscreen.
- Repeat launch from another focused application focused the same game PID.
- An update check during gameplay was rejected before downloading.
- Tile resizing changed the window from 1701×1390 to 1601×1390; reversing the
  resize restored the original size.
- The compositor fullscreen dispatcher used by Omarchy's fullscreen binding
  entered fullscreen state 2 and returned to state 0.
- Workspace movement completed a 1 → 9 → 1 round trip with the same game PID.
- Closed the test session normally; the session marker was removed and the
  launcher lock was released. Test dialogs and file-manager windows were closed.

### Published-artifact verification and scope

Re-verified previously downloaded v0.2.0 installer, manifest, and launcher
attestations with the explicit repository/workflow/tag/source/runner policy. All
three passed, and installer/launcher hashes matched the authenticated manifest.

This validates the local candidate on an existing installation and real v0.2.0
artifact provenance. Clean-machine dependency provisioning, a newly published
candidate release and its attestations, and a future changed-MSI upgrade still
need their own acceptance runs. Fresh browser sign-in and an online match were
not repeated in this smoke test; successful online sessions remain user-confirmed
as recorded above.

## Fresh isolated application installation — 2026-10-15

Tested the same candidate with a new HOME and XDG data/cache/config/state
directories under the ignored `build/` directory. No existing game prefix,
runtime selection, installer receipt, download cache, or login state was copied.
The test reused installed system dependencies and did not install system packages.

- Launcher self-installation generated separate desktop files, callback
  registration, and Lua integration files. The live Hyprland configuration was
  not replaced or reloaded with the sandbox configuration.
- Setup downloaded the pinned GE-Proton archive from scratch and verified
  SHA-256 `c5448b76a230384e2d7bc6beb5ccb97bafb7e2c3b6c527cb03a1a546bbcb00a0`.
- UMU downloaded Steam Linux Runtime 4, checked its archive digest and mtree, and
  initialized a new prefix. Required Visual C++ DLL checks passed.
- Setup downloaded and authenticated the official MSI, installed the game, and
  wrote a new `setup` receipt for installer SHA-256
  `53bacfb0ecb71e10390bd8cd3cdc2ae75df67329a355e9d1413efb9680416a12`.
- A second setup was a no-op and retained the same receipt hash.
- Fourteen sandbox diagnostic checks passed. The Hyprland-session check was
  intentionally unavailable because its session signature was cleared during
  isolated integration; live configuration validation was consequently skipped.

### Fresh sign-in and callback routing

The first login attempt exposed a test-isolation mistake: the normal browser
invoked the normal installation's callback handler while the fresh game used a
different HOME/XDG prefix. Browser authentication completed, but delivery could
not find the sandbox session, leaving the game at Retry.

For the corrected test, a temporary, uniquely named desktop callback handler
invoked the launcher with the sandbox environment. The user completed browser
authentication, delivery to the isolated prefix was recorded without retaining
callback arguments, and the same fresh game reached its signed-in home screen.
No production callback-code change was needed for this routing correction.

### Restoration and scope

The original callback association was restored, the temporary desktop handler
was removed, and the normal MIME settings matched their original SHA-256 exactly.
The original launcher, main Lua config, managed rules, runtime profile,
installation receipt, and prefix registry also retained their pre-test hashes.
All 16 diagnostics passed for the normal installation afterward.

The isolated game was closed normally and its session lock released. The isolated
application data was removed after verification. No personal installation was
uninstalled or reset.

This verifies fresh application provisioning and first browser sign-in on the
existing host. It does not establish clean-OS dependency provisioning, availability
of a supported verifier package channel, or publication of the candidate release.

## Published v0.3.0 verification — 2026-10-15

- Release: <https://github.com/hucKOder/omarchy-ptcgl/releases/tag/v0.3.0>.
- Tagged source: `0346263b6fc5bf02746d7376570bc7e4f58c18e6`.
- The [release workflow](https://github.com/hucKOder/omarchy-ptcgl/actions/runs/38751789084)
  and both source/tag CI runs passed. The release is stable and selected as Latest.
- Downloaded all four published assets. Installer, manifest, and launcher
  attestations passed the explicit repository/workflow/tag/source/runner policy.
  Manifest identity/platform fields and installer/launcher hashes matched.
- Published installer SHA-256:
  `40a29862fbceb33a3d9967ab597f96c781a56eb3f580fce4c92038a8d86d0e69`.
- Published launcher SHA-256:
  `771b50edb1e32af4e422298a268e6e743e86b076c709f1a3f823a4f9f76ab6c7`.

The published installation block matched the reviewed generated instructions,
including the explicit version and source commit. Executed that block with the
documented `--launcher-only` option. It downloaded and authenticated the installer,
authenticated release metadata and the launcher, and refreshed the existing
launcher/integration successfully.

The installed executable matched the published launcher hash and reported
`ptcgl-omarchy 0.3.0`. All 16 diagnostics passed. Hashes of the game registry,
installation receipt, selected runtime profile, main Hyprland Lua config, and
managed window rules were identical before and after the upgrade.

This establishes the published download/authentication path and launcher-only
upgrade on the existing host. The fresh application installation/sign-in test
above used the equivalent local candidate before version stamping. Clean-OS
dependency provisioning, interrupted-install recovery improvements, and a future
changed-MSI upgrade remain separate follow-up work.

## Published v0.4.0 verification — 2026-10-15

- Release: <https://github.com/hucKOder/omarchy-ptcgl/releases/tag/v0.4.0>.
- Tagged source: `45e4398fc0477be23d946a727f13bb5c41fec9f1`.
- [Source CI](https://github.com/hucKOder/omarchy-ptcgl/actions/runs/38758723811),
  [tag CI](https://github.com/hucKOder/omarchy-ptcgl/actions/runs/38758943854), and
  the [release workflow](https://github.com/hucKOder/omarchy-ptcgl/actions/runs/38758943643)
  passed. The release is stable and selected as Latest.
- Downloaded all four published assets. Installer, manifest, and launcher
  attestations passed the explicit repository/workflow/tag/source/runner policy.
  Manifest identity/platform fields and installer/launcher SHA-256 hashes matched.
- Published installer SHA-256:
  `7281d7db837ef3b137057059790a5bcdaf343afbcc571a5c02a6366b93118868`.
- Published launcher SHA-256:
  `dd523b6c0e219309259f34c7d2af6d445608a192ae35de75805c7d2cdeddc3dc`.
- The binary reports `ptcgl-omarchy 0.4.0`; build metadata identifies Go 1.27.2,
  the tagged source commit, `linux/amd64`, `CGO_ENABLED=0`, and an unmodified
  release checkout. Its `licenses` output matches the committed dependency notices.

Executed the README's real source-bootstrap command with `--launcher-only` in
temporary HOME/XDG directories. Package-management and browser-handler commands
were isolated stubs; release discovery, source-tag resolution, network downloads,
attestation verification, the published installer, and launcher self-installation
were real. `GH_TOKEN`, `GITHUB_TOKEN`, and `GH_CONFIG_DIR` were unset. The flow
selected v0.4.0 and its full source commit without manual substitution, installed
the exact published launcher bytes, and generated the desktop entries and Lua
rules under the temporary paths. No game prefix was created by launcher-only mode.
The published notes contain the expected commit-pinned one-line command.

Both the published binary and sandbox-installed copy successfully verified the
cached official Pokémon MSI, including its content/signature, exact publisher,
certificate chains, timestamp, and current online CRLs. The installed-copy check
used a failing `osslsigncode` stub on PATH, establishing that the built-in verifier
does not depend on that external tool.

Local `make check`, `make test`, `make build`, and `go mod verify` passed before
publication. Source vulnerability analysis found no reachable vulnerabilities;
the unused-module advisories reported by the scanner were retained in its output.
The release was built with the stable Go toolchain, while CI also checked the
minimum Go version declared by the project.

This verifies the published authentication path, isolated launcher installation,
and real MSI verification. Actual package provisioning on clean Omarchy, a fresh
full-game installation with v0.4.0, and new gameplay acceptance remain separate
manual validation tasks.

## Security-candidate acceptance — 2026-10-22

Tested source based on `e6dcd334432ad1a091446e0db9e6527bb1526b87`, with the
uncommitted security changes, as `0.4.1-security-candidate` using Go 1.27.3.
The final installed candidate and security-only review checkout produced identical
launcher bytes, SHA-256:

```text
02b4c2876fd0c07b4299668036c189553064a1077940645becd41cfcfa7415f8
```

This identifies the development candidate; a committed/tagged release has its own
VCS metadata and requires hosted build comparison and artifact authentication.

### Automated checks and targeted review fixes

- Go formatting/vet/unit tests, race-enabled tests, module integrity,
  ShellCheck, and actionlint passed. The full development checkout's companion
  JavaScript and QML/manifest checks also passed.
- Source-aware vulnerability analysis with Go 1.27.3 found zero reachable
  vulnerabilities; 22 module-level findings with no imported affected packages
  remain separate from reachable application findings.
- Fixed two release-policy false positives from the saved independent review:
  unresolved default-branch aliases are rejected, and required `test` and
  `vulnerabilities` contexts must be bound to GitHub Actions app 15368.
- Strengthened compromised-CA verification with public certificate-only fixtures,
  explicit test roots, a fixed valid verification time, and the specific
  compromised-key chain-filtering error. An offline comparison accepted both
  fixtures with Go 1.27.2 and rejected both with Go 1.27.3. Optional real downloader
  tests also passed with explicit test roots, so root removal cannot mask a
  missing runtime key denylist.

### Installed upgrade and real provisioning

- Before installation, saved a private backup of the original launcher,
  integration/state files, and game prefix. Evidence and backup are under
  `~/.local/state/ptcgl-omarchy/release-acceptance-20261022/`, outside the checkout.
- Installed the candidate through its actual self-installation command and
  confirmed the installed binary hash. All diagnostics passed. Installation
  preserved existing registry/settings, runtime selection, receipt, MIME settings,
  and main/managed Hyprland configuration hashes.
- The installed candidate downloaded and authenticated the current official MSI.
  Its hash remained
  `53bacfb0ecb71e10390bd8cd3cdc2ae75df67329a355e9d1413efb9680416a12`;
  the update check preserved the receipt and registry hashes.
- Fresh HOME/XDG directories with spaces exercised a real Proton download,
  checksum verification, the new extractor, Steam Linux Runtime download and
  integrity checks, prefix initialization, and authenticated MSI installation.
  A second setup was a no-op and preserved the fresh receipt. System dependencies
  were already installed; clean-OS package provisioning was not exercised.
- Real desktop launch reached the existing signed-in home screen. The actual game
  process excluded injected synthetic `GH_TOKEN`, `OPENAI_API_KEY`, and
  `AWS_SECRET_ACCESS_KEY` variables while retaining its display environment.
- Repeat launch focused the same PID. Maintenance was blocked before download
  while playing. Tiling, resize/restoration, compositor-internal fullscreen and
  restoration, and a workspace 1 → 9 → 1 round trip passed.
- The installed companion detected the candidate as ready. Its Play command and
  the app-menu entry both invoke `~/.local/bin/ptcgl-omarchy app`.

### Host TLS tools and release configuration

- Before remediation, active `curl` and GitHub CLI 2.102.0 (Go 1.27.1) accepted
  the original-root compromised-CA test host. Both already rejected the alternate
  test chain because its issuer was not trusted.
- Authenticated and installed Arch's signed `ca-certificates-mozilla 3.131.1-1`
  update. Updated the active GitHub CLI through mise to 2.102.1, built with Go
  1.27.3; this respected the existing minimum-release-age policy. Its executable
  matched the authenticated upstream archive. Both tools now reject both public
  test hosts with certificate-verification failures.
- Activated no-bypass rulesets explicitly protecting `main` and `v*` tags.
  Configured the `release` environment for maintainer approval, no admin bypass,
  and tag-only deployment. The real read-only policy check passed.
- The owner confirmed this is a solo-maintainer project. PRs require CI and
  resolved review threads with zero mandatory approvals; release approval is by
  the owner with self-review allowed. This is not independent human review.

### Login issue discovered during manual acceptance

The user reported a login error, followed by initialization reaching 61/61 and
stalling. A companion restart initially stalled again. Launching from the normal
Omarchy app menu then reached the signed-in game, and a subsequent companion
launch also worked. The installed binary remained the candidate identified above.

No launcher root cause or corrective authentication patch has been established.
The menu and companion use the same executable and `app` argument. Existing game
exception logs predated the failing session and cannot explain this occurrence.
On a subsequent explicit clean retry, the user confirmed successful fresh browser
login/callback delivery and a completed online match with working graphics/audio.
Retain the initial failure and recovery in this record; do not describe it as a
confirmed source-level fix. Hosted release/rebuild verification and new-release
authenticated upgrade remain outstanding.

### First hosted release-candidate run

The immutable `v0.4.1-rc.1` tag identifies
`0bb59ce851e52c0b9b23f767d399a978eff3e1ee`.
[Its workflow](https://github.com/hucKOder/omarchy-ptcgl/actions/runs/39778213696)
passed vulnerability analysis, build/tests, and separate-runner exact rebuild.
The policy job failed closed because GitHub hides `bypass_actors` from its
read-only token; publishing was skipped and no RC release assets were published.

The follow-up explicitly distinguishes the hosted public-metadata gate from the
owner's default full policy audit. The owner must pass that full audit, including
visible empty bypass lists, before release-environment approval. Regression
fixtures cover hidden lists, rejection of known bypass actors in either mode,
missing CI, and unknown options. The failed tag is retained rather than moved.
