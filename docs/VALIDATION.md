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

## Published v0.4.1 verification — 2026-10-22

- Stable release: <https://github.com/hucKOder/omarchy-ptcgl/releases/tag/v0.4.1>;
  selected as Latest.
- Tagged source: `06345c9bb9a5c2502770fca34aec3e45f3eaed9d`, merged through
  [the security PR](https://github.com/hucKOder/omarchy-ptcgl/pull/1) and
  [the hosted-visibility fix](https://github.com/hucKOder/omarchy-ptcgl/pull/2),
  with required CI passing under the active branch ruleset.
- [RC2 workflow](https://github.com/hucKOder/omarchy-ptcgl/actions/runs/39779613967)
  passed every gate and published a prerelease without replacing Latest.
- [Stable release workflow](https://github.com/hucKOder/omarchy-ptcgl/actions/runs/39780062584)
  passed public policy metadata, source ancestry, vulnerability analysis,
  shell/unit/race checks, build, separate-runner exact reproduction, and protected
  publication. Before each environment approval, the owner's full authenticated
  no-bypass policy check also passed. No administration credential was stored in
  Actions.
- Downloaded all four stable assets. Installer, manifest, and launcher
  attestations passed the exact repository/workflow/tag/source/signer/issuer/
  hosted-runner policy. Identity/platform fields and both artifact hashes matched.
- Published installer SHA-256:
  `7281d7db837ef3b137057059790a5bcdaf343afbcc571a5c02a6366b93118868`.
- Published launcher SHA-256:
  `e33b58f83dcf412810f7deb2832c7efe3c3c31feafd1933ef4649a83e37a1897`.
- The launcher reports `ptcgl-omarchy 0.4.1`, Go 1.27.3, Linux/amd64,
  `CGO_ENABLED=0`, the tagged source, and an unmodified checkout. A clean local
  tagged-source build also matched the published launcher bytes exactly.
- The published binary authenticated the existing official MSI with current
  complete-chain revocation evidence.

Executed the real commit-pinned source bootstrap with explicit v0.4.1/source
selection and `--launcher-only`, using real downloads, attestations, installer,
dependency checks, and self-installation. The downloaded bootstrap matched the
reviewed source bytes. The installed executable matched the published launcher
digest and reported v0.4.1. All diagnostics passed, and the installed launcher
successfully authenticated a fresh official-installer update check.

Before/after hashes of the registry, installation receipt, runtime profile,
integration state, MIME preferences, and main/managed Hyprland configuration were
identical across the published upgrade and update check. The candidate's fresh
provisioning/desktop checks and user-confirmed fresh login/online match are
recorded above, including the initial 61/61 stall and subsequent clean retry.

Clean-OS dependency provisioning, a future changed-MSI upgrade, custom overlays,
and filesystem/network sandboxing remain separate follow-ups. The installed
security release and its authenticated upgrade acceptance are complete.

## UX milestones 1–3 — isolated implementation checks, 2026-10-29

Implemented the live status/startup/compact companion milestone, download progress
and interrupted-install recovery, and guided diagnostics/component update overview
in the working checkout. This is **not** acceptance of a published release or an
installed desktop upgrade.

- `make check`, `make test` (race-enabled), and `make build` passed with Go 1.27.3.
- `make test-plugin` passed all 11 JavaScript contract tests; `make check-plugin`
  passed Omarchy manifest validation and zero-warning QML lint with Qt 6.12.0.
- `make check-shell` and module integrity verification passed.
- `make security` found no reachable vulnerabilities (one module-only advisory
  remains reported by the scanner, outside imported vulnerable packages/symbols).
- An isolated built-binary `status --json` smoke check emitted a schema-1
  `setup-needed` / `idle` response and created no HOME/data/state directories.
- New launcher regressions cover actual flock ownership and process-start/boot
  identity, stale/unknown operation metadata, orphaned owned windows, repeat launch
  during startup, known/unknown-length downloads, cache-preserving cancellation,
  non-cancellable verifier/MSI phases, runtime re-staging and custom-runtime
  preservation, interrupted/unrecorded MSI installation with leftover executables,
  confirmed/cancelled fixes, private report saving/copying, and update-component
  separation. Startup dialog dismissal is tested against a local stub process,
  not the real Pokémon client.

Tests use temporary HOME/XDG paths, loopback TLS servers, stub dialogs, and local
process fixtures. The built candidate and source companion were not installed or
enabled. The existing game prefix, browser handler, desktop entry, and Hyprland
configuration were not changed. Build version stamping is not a release: the
published v0.4.1 binary does not include these features, and the new companion
requires a matching launcher with status schema 1.

Still required before publication: closed-game installed-candidate acceptance,
real startup focus/dialog behavior, slow/rapid launch and repair flows, keyboard /
vertical-bar / multi-monitor interaction, shell-restart survival, fresh browser
sign-in and gameplay, clean-OS first-time tester coverage, and authenticated
published-artifact acceptance under the existing release protections.

## UX milestones 1–3 — live desktop smoke test, 2026-10-29

Tested the working checkout as **0.5.0-dev** on the user's existing Omarchy
4.0.6-1 desktop, Hyprland 0.56.2, Qt 6.12.0, and the existing managed
GE-Proton11-7 runtime. Two displays were available, including a portrait display.
Candidate SHA-256:

```text
4ca3343e20b5c7744662bcb276ae32aff920b1fbd075ea5ad5754418c3f98ea8
```

### Temporary deployment and preservation

- Confirmed no game session was running before deployment. Backed up the installed
  launcher, companion files, desktop integration, bar configuration, receipts,
  and prefix registry files in a private test directory.
- Temporarily installed the candidate and matching source companion. Immediately
  after launcher installation, only the launcher executable and main desktop entry
  differed: registry state, runtime selection, installation/integration receipts,
  MIME preferences, and main/managed Hyprland configuration retained their hashes.
- Existing login state was retained: the real game reached its signed-in home
  screen. No match was started, no sign-out was requested, and no fresh browser
  authentication was performed.
- The user explicitly approved normal game shutdown to finish maintenance checks
  and restoration. No game process was forcibly terminated.

### Passed live checks

- Actual compact companion displayed the running state and **Focus game** action.
  Keyboard expansion of Maintenance and Escape dismissal worked. Modifying
  maintenance actions were disabled while the game ran.
- Repeated launch focused the same game window/PID. A separate launch during
  startup also returned successfully without creating another game session.
- The real app-menu desktop entry launched the game. Observed `idle → starting →
  running`, with startup feedback, a tiled XWayland window, and no initial fullscreen.
- Fullscreen round trip and workspace move/return succeeded. Horizontal tile
  resizing changed the window from 846×1390 to 926×1390. An initial height-only
  resize was ineffective because the tile occupied the full workspace height;
  that was an unsuitable test axis, not evidence of a launcher regression.
- The game retained its PID and running status through an Omarchy shell restart.
  Normal shutdown returned status to idle and removed the play-session marker.
- Companion summon mapped to the correct selected monitor on both displays.
- Backend `update --check` was rejected during gameplay before any installer
  operation; the successful installation receipt was unchanged.
- Closed the real download progress window during an active transfer. The command
  reported cancellation, preserved the verified cache and installation receipt,
  and removed temporary transfer files.
- A subsequent real `update --check --gui` downloaded the 187.4 MiB official MSI,
  showed measured progress and a separate non-cancellable verification phase,
  authenticated it, and displayed the unchanged-installer result. The receipt did
  not advance; Windows Installer was not invoked.
- The actual updates overview and diagnostic report opened and dismissed normally.
  The repair confirmation was cancelled, with no repair applied.
- Saved support reports through both CLI and the real graphical file chooser.
  Both exports were mode 0600 and omitted personal paths and callback URLs.

### Deployment observation

Updating the existing companion files, rescanning, and disabling/re-enabling did
not invalidate the running shell's cached QML/JavaScript: the first popup still
showed the old controls. `omarchy restart shell` loaded the matching new controls.
This is the local-update cache caveat already documented in the companion guide;
do not count a successful rescan alone as proof that new companion code is active.

### Restoration and limits

Restored the original **v0.4.1** launcher, desktop entry, companion, and exact bar
layout, then restarted the shell to reload the restored companion. All protected
non-registry files matched their original hashes, the original active window was
restored, the original companion opened, all **15** current diagnostics passed,
and Hyprland reported no configuration errors. No game or test dialogs remained.
Wine's `system.reg` and `user.reg` changed during normal game sessions; those live
registry changes were deliberately retained, not rolled back. The game prefix was
never reset and the selected Proton profile was not changed. Temporary screenshots
were removed after review; none were added to the repository.

Still unverified: fresh browser sign-in, a completed match with the candidate,
vertical-bar layout, intentionally interrupted real MSI/runtime installation,
application of a changed MSI or repair, clean-OS provisioning, and a new published
release's artifact acceptance. Clipboard-copy behavior remains fixture-tested;
the user's clipboard was not intentionally replaced for this test.

## v0.5.0-rc.1 preparation — 2026-10-29

Prepared an opt-in prerelease of the UX milestones and matching companion, not a
stable promotion. The default bootstrap and companion install/upgrade helper still
select stable v0.4.1; RC testers must explicitly use the candidate's commit-pinned
release instructions. The companion requires status schema 1 from that candidate.

- `make check`, `make check-shell`, `make test-plugin`, `make check-plugin`, and
  `make build` passed with Go 1.27.3 and candidate version `0.5.0-rc.1`.
- Fresh `go test -race -count=1 ./...` passed; module integrity passed and
  `make security` found zero reachable vulnerabilities (one module-only finding).
- The owner's full authenticated release-protection audit passed during readiness
  review; it must pass again before approving the candidate's publication job.
- The prior live desktop test above remains the desktop evidence; release
  preparation did not deploy a launcher/companion or change the installed game.
- An unreferenced local `tiling-demo-21s.mp4` was left untouched and excluded from
  the release source.

Hosted CI, exact rebuild, protected publication, and downloaded-artifact acceptance
are separate release gates. Fresh login/gameplay, real repair/interruption recovery,
vertical-bar interaction, and clean-OS first-install acceptance remain open before
stable promotion or marketplace submission.

## Published v0.5.0-rc.1 verification — 2026-10-29

- Prerelease: <https://github.com/hucKOder/omarchy-ptcgl/releases/tag/v0.5.0-rc.1>.
  Stable **v0.4.1 remains Latest**; no stable promotion or marketplace submission.
- Immutable annotated tag source: `fd3136382613d5b8028fbad1c9cfea7fbb1199ae`,
  merged through [PR #5](https://github.com/hucKOder/omarchy-ptcgl/pull/5) with required
  checks passing and no branch-protection bypass.
- [Merged-source CI](https://github.com/hucKOder/omarchy-ptcgl/actions/runs/40902040361),
  [tag CI](https://github.com/hucKOder/omarchy-ptcgl/actions/runs/40902374313), and the
  [release workflow](https://github.com/hucKOder/omarchy-ptcgl/actions/runs/40902374298)
  passed. The release included policy/source-ancestry, vulnerability, shell/unit/race,
  separate-runner exact-rebuild, and protected-publication gates. The owner's full
  authenticated no-bypass audit passed again immediately before environment approval.
- Downloaded all four assets. Installer, manifest, and launcher attestations passed
  the exact repository/workflow/tag/source/signer/issuer/hosted-runner policy. Manifest
  identity/platform fields and both artifact hashes matched.
- Published installer SHA-256:
  `7281d7db837ef3b137057059790a5bcdaf343afbcc571a5c02a6366b93118868`.
- Published launcher SHA-256:
  `bfc75de58ddcc924505d1f9f0057f2cf46e5f49a0121899ef60c58d92a38897e`.
- A separate clean local worktree at the tagged source rebuilt the exact published
  launcher bytes. The binary reports `0.5.0-rc.1`, Go 1.27.3, Linux/amd64,
  `CGO_ENABLED=0`, the tagged source, and `vcs.modified=false`. Embedded licenses and
  published commit-pinned installation instructions matched the generated source.
- Workflow actionlint validation passed with actionlint v1.7.7. Hosted annotations
  about Node.js 20 action deprecation and the Ubuntu runner-image migration remain
  maintenance follow-ups; neither caused a gate failure.

### Authenticated isolated installation and reinstall

Executed the real commit-pinned bootstrap with explicit RC/source selection and
`--launcher-only`, in fresh HOME/XDG paths containing spaces. The downloaded bootstrap
matched reviewed source bytes. The test environment contained no GitHub credentials,
desktop/session-bus variables, or inherited runtime settings. Package provisioning
and browser-handler registration used isolated fixtures; HTTPS downloads, release
authentication, the published installer, and launcher self-installation were real.
Live compositor, game/runtime, and graphical-dialog commands were blocked by fixtures.

- The installed launcher matched the authenticated published hash and reported the
  expected version. Both desktop entries passed `desktop-file-validate`, including
  the new Versions & updates action and confirmed Repair command.
- Schema-1 status reported `setup-needed` / `idle`; launcher-only mode created no
  game prefix, installer receipt, or runtime selection.
- A second real authenticated launcher-only installation preserved byte-for-byte
  synthetic prefix/settings data, an existing synthetic receipt, the selected
  runtime profile, integration state, callback fixture, and main/managed Lua files.
  This is preservation/orchestration coverage, not real Windows Installer acceptance.
- A separate fresh-HOME `status --json` invocation produced the expected schema-1
  response without creating that HOME or any application directories.
- The published binary successfully authenticated the existing official MSI,
  SHA-256 `53bacfb0ecb71e10390bd8cd3cdc2ae75df67329a355e9d1413efb9680416a12`,
  with current complete-chain revocation evidence. This was read-only verification;
  it did not run Windows Installer or change the real cache.

### Scope and remaining acceptance

The real installed launcher retained the exact v0.4.1 hash recorded above. No
candidate launcher/companion was deployed to the user's desktop, no callback or
Hyprland configuration was changed there, and no real prefix was modified. Local
acceptance logs and fixtures are retained outside the repository under
`/tmp/opencode/ptcgl-v0.5.0-rc.1-acceptance.gwXkjjsx/`.

The prior 0.5.0-dev existing-installation desktop smoke remains the desktop evidence.
Fresh browser login, a completed match, actual repair and interrupted MSI/runtime
recovery, vertical-bar interaction, and clean-OS provisioning still need acceptance
before stable promotion or marketplace submission. A changed-MSI upgrade also needs
its own record when a newer official installer is available. The published RC's
authentication and isolated launcher-install acceptance are complete, not a claim
that these remaining real-desktop/clean-OS checks passed.

## Installed v0.5.0-rc.1 and user acceptance — 2026-10-29

After the published-artifact checks above, the user requested installation of the
candidate to perform the logout/login and play-session test personally.

- Confirmed the game was closed and installation status was `ready` / `idle`.
  Reauthenticated installer, manifest, and launcher against the exact RC/source
  policy and hashes before installing.
- Saved a private launcher/integration/companion backup, then ran the published
  authenticated installer with explicit `v0.5.0-rc.1`, source
  `fd3136382613d5b8028fbad1c9cfea7fbb1199ae`, and `--launcher-only`.
  No game setup, repair, or runtime update was invoked; dependencies were present.
- Updated the installed hand-copied companion to the exact tagged source and
  restarted the Omarchy shell to clear cached old QML/JavaScript. The existing
  bar layout was retained; no plugin source edits were made for the local upgrade.
- Installed launcher SHA-256 matched the published RC:
  `bfc75de58ddcc924505d1f9f0057f2cf46e5f49a0121899ef60c58d92a38897e`.
  All 15 diagnostics passed, companion validation passed, and Hyprland reload
  reported no configuration errors.
- All 17 protected file snapshots were unchanged after installation, including
  the existing registry files/game executable, receipt, runtime selection,
  configuration, MIME handler preferences, managed/main Hyprland configuration,
  and bar configuration. Missing optional files remained absent. The main desktop
  entry and companion intentionally gained the new controls.
- Backup and installed verification are private under
  `~/.local/state/ptcgl-omarchy/upgrade-v0.5.0-rc.1-u4o5jdrt/`.

The user subsequently reported **“everything works fine. We can do release”** in
response to the planned logout/browser-login and play-session test. This records
user-confirmed acceptance, including the requested browser-return/gameplay/graphics
and audio check, not an agent-observed match or a fresh-prefix provisioning test.
No source-level authentication fix or additional runtime change was needed.

The maintainer approved stable **v0.5.0** promotion on this evidence. Real repair and
interrupted MSI/runtime recovery, vertical-bar interaction, clipboard-copy behavior,
slow/failing startup variants, and clean-OS/first-time-tester coverage remain unverified
follow-ups accepted for this release; a changed-MSI upgrade still needs a newer
official installer. They are not silently marked passed. Stable publication must
still pass the existing protected-main, vulnerability, exact-rebuild, approval, and
artifact-authentication gates. Release preparation does not replace the installed
RC or touch an active game session.

### Stable preparation checks

The only runtime source difference from the tested RC is `Version = "0.5.0"`;
launcher behavior, companion code, installers, dependency pins, and release gates
are unchanged. Stable-facing documentation and generated notes now describe the
successful user test and retain the unverified follow-ups explicitly.

`make check`, `make check-shell`, all 11 companion contract tests,
`make check-plugin`, `make build`, and fresh `go test -race -count=1 ./...` passed.
Module integrity passed; `make security` found zero reachable vulnerabilities
(one module-only advisory). The full authenticated release-protection audit passed
during preparation and must pass again before protected publication approval.

## Refactor desktop acceptance and portrait startup failure — 2026-10-29/30

The first refactor smoke test was not a successful login/gameplay acceptance:
the user reported a second black window after browser login and a stall at 61/61.
A retry with the candidate handling both launch and callback also stalled. A
callback worker returning successfully proved delivery was started, not that the
game completed authentication or reached its menu.

A private diagnostic build changed only the normal `-nolog` arguments to Unity
logging through captured stderr, filtering sensitive output before saving it.
At the stall Unity reported that scene `MainMenu_3` could not be loaded. Read-only
inspection of the installed client's scene list found `MainMenu_3_Landscape`,
and its scene-selection method chose `MainMenu_3` when screen width was less than
height. The failing tiled window measured 701×1248.

After a reboot, a fresh diagnostic candidate passed all 15 doctor checks. A
temporary, game-only floating resize changed the initial 701×1248 tile to a
1280×721 landscape window before initialization completed. The user confirmed:
“no more stuck i am in game can play.” No portrait-scene load error appeared in
this run. This successful attempt reused existing login state; no new browser
callback was observed, and a completed online match was not recorded.

The installed launcher, saved window rules, runtime selection, and game data
were not repaired or replaced. No cache was cleared. The diagnostic launch used
temporary callback routing with restoration to the installed handler. The game
was left open for the user. A permanent portrait-safe launch/layout policy is
still outstanding; keep the game window wider than tall in the meantime. This
result does not validate arbitrary portrait tile sizes or every scene transition.

## Permanent landscape fallback and normal candidate — 2026-10-30

The working source now requests windowed 1280×720 for both Play and callback
delivery, retaining `-nolog`; the detached callback `Start` behavior described
here was later superseded by the bounded helper cleanup below. Play owns a
100 ms landscape observer until the game exits. It leaves landscape/square tiles
alone; a portrait game window in this prefix is taken out of portrait fullscreen
if necessary, floated, resized to 1280×720, and given `keep_aspect_ratio` for mouse
resizing. The observer handles every matching owned window, including subsequent
callback windows, rather than stopping at the first window/menu. Each mutation
rechecks address, PID and prefix ownership. Explicit foreign prefix variables
cannot be overridden by a game-path argument. Other applications, prefixes and
workspaces are not targeted; the correction does not dispatch focus.

**Policy limits:** corrected windows stay floating for the session; they are not
automatically retiled. A user may retile into a landscape layout. Keyboard resize,
retiling and portrait-monitor fullscreen are corrected reactively, not atomically.
Keep the game wider than tall before authentication and menu/match transitions;
arbitrary portrait tiling is not supported. A scene selection during an intentional
portrait resize can still race the observer. Protection requires the launcher and
a working compositor to remain alive. Compositor failures appear as a fixed,
privacy-safe warning in companion status and do not stop the game. Transient
failures are retried; unconfirmed observer subprocess cleanup propagates through
Play and retains the existing persistent safety guard.

No saved Hyprland rules or user Hyprland configuration were changed. The legacy
wiki rule URLs returned 404; current official rule/dispatcher documentation was
read from `hyprwm/hyprland-wiki` under `content/configuring/core/`, with dispatcher
availability checked against installed `/usr/share/hypr/stubs/hl.meta.lua`, Omarchy
tiling helpers, and the official Hyprland v0.56.2 implementation. In particular,
floating uses idempotent `action = 'enable'`, not toggle.

Regression tests cover landscape no-op, narrow startup, later callback windows
and portrait/fullscreen scene layouts, invalid/reused/disappearing window identity,
foreign ownership, compositor query/JSON/dispatch/rejection/clamping failures,
observer cancellation/join/recovery, retained cleanup guards, and matching private
launch/callback flags. `make fmt`, `make fmt-check`, `make lint`, `make check`, fresh
`go test -race -count=1 ./...`, `make check-shell`, all 11 `make test-plugin` tests,
read-only `make check-plugin`, and `git diff --check` passed. A release-style
`CGO_ENABLED=0`, readonly-module, trimpath, stripped build passed in a unique
`/tmp/opencode` directory (existing `build/` comparison binaries were preserved).
`make security` verified module integrity and found zero reachable vulnerabilities,
zero imported-package vulnerabilities, and one module-only advisory.

### Installed update and live evidence

Before launching, the user's updated installed binary was inspected rather than
replaced: version 0.5.0, clean source revision
`72fc50a2956ee1954f7f74d05ce42fa2d65f9c4e`, SHA-256
`eb6b817b5fce8684b9ba34badf76cf104179b95551ca5544d1204501f6f43d30`.
This differs from the refactored working source and its landscape candidate.
Both installed and candidate preflight reported ready/idle, all 15 doctor checks
passed, and there was no game window, cleanup guard, session/operation marker or
pending installer. No repair, reinstall, credential reset, cache wipe or runtime
upgrade was performed.

At 07:44:57 UTC, normal unmodified-source candidate Play was started from
`/tmp/opencode/ptcgl-landscape-normal-hF0mja49/ptcgl-omarchy`, SHA-256
`e77ade4e7613e60b53c5f92ed03ecfea51991262171931b4ef3c09c785445b24`.
UMU runtime updating was disabled for this test only. There was no diagnostic
overlay, Unity log override or external geometry watcher. The private supervisor
discards process output and records only fixed session/callback events, never URLs.
The normal launcher itself produced a floating **1280×721** XWayland game window
on monitor 1, workspace 2. The first pre-correction dimensions were not captured.
Status reports running with maintenance disabled and no layout warning; that alone
does **not** prove successful authentication, main-menu arrival or gameplay.

At the initial checkpoint the candidate game was left open for user testing. Fresh browser authentication,
menu/match scene acceptance and a completed online match remain **pending user
confirmation**; no callback dispatch had been observed at this checkpoint. Existing
stored login must not be counted as fresh browser authentication. Temporary
callback routing points to this same candidate while it is open; the installed
callback desktop file is untouched. The supervisor restores only its own MIME
association on normal session exit, preserving unrelated concurrent preference
edits or a concurrent replacement of that association. Restoration is pending
while this test session remains open. No launcher installation, commit or release
publication was performed.

At the final read-only checkpoint there was exactly one owned game window, still
1280×721 and floating, with no layout warning. `/proc` confirmed the active launcher
is the candidate above, not the installed binary. The installed binary hash and
callback entry remained unchanged. The active Play lock, session marker and
prearmed operation guard are intentionally present until Play exits; they are not
a stale guard to delete or bypass. A dry comparison confirmed that restoring just
the temporary MIME key reproduces the original preferences bytes when no concurrent
preference edits occur.

### Normal candidate session completed — 2026-10-30

The private event record subsequently observed a normal candidate callback dispatch
at **07:49:51 UTC**. This confirms that callback delivery was started with the
matching candidate, not successful authentication or menu arrival. No callback URL
or authorization value was recorded.

Normal Play exited with status **0 at 08:24:57 UTC**, without an agent-issued close
or restart. The supervisor restored `ptcgl-omarchy-callback.desktop` as the original
default handler and removed its uniquely named temporary desktop entry. A read-only
check confirmed that MIME preferences match their original bytes exactly, installed
binary and callback-entry hashes are unchanged, status is ready/idle, and operation,
Play-session, cleanup-guard and pending-installer markers are absent. No new session
was started. The earlier open-session checkpoint above is historical; restoration
is now complete. Fresh-login success, menu/match scene acceptance and a completed
online match still require user confirmation and are not inferred from callback
dispatch or clean exit.

## Fresh browser return leaves a persistent helper — 2026-10-30

The user subsequently reported: **“I logged out and logged back in. I am back in
game but now there is a black window opened.”** This is user-confirmed fresh
browser sign-in/menu arrival in the normal candidate above, **not full acceptance**:
the duplicate black window remains a failure, and no completed online match has
been confirmed for this candidate.

### Read-only live findings

The temporary supervisor recorded one `callback-dispatch-started` event at
07:49:51 UTC. The association still targets
`ptcgl-omarchy-validation-hF0mja49.desktop`, whose wrapper invokes the same candidate
SHA-256 recorded above. The installed v0.5.0 binary retains its recorded hash;
there is no evidence of a mismatched installed callback handling this return.

Inspection checked process UID, executable identity, the game executable argument,
and resolved prefix ownership before reporting game-window metadata. Only fixed
callback-scheme presence was retained, never callback arguments, tokens or raw
environment data. Both windows are owned XWayland `steam_app_0` clients on monitor
1, workspace 2:

| Role / evidence | PID / address | Start UTC | First observed layout |
| --- | --- | --- | --- |
| Original Play game, no callback argument | 43050 / `0x55dc02ec6910` | 07:45:04 | floating 1280×721 |
| Callback-launched game executable, callback-scheme argument present | 49441 / `0x55dc030952c0` | 07:49:51 | tiled 701×619 |

The second process has Unity/Mono mapped and remained alive for more than twelve
minutes after the callback. Between the 07:53:39 and 08:02:36 observations its CPU
time advanced at approximately one full core. It is a persistent game-engine
process, not merely the short-lived browser URI wrapper or a launcher dialog.
The original game PID/address remained present throughout.

The helper was already landscape and tiled at the first inspection, so the
landscape observer has no correction to perform in that state. By 08:02:36 the
same helper was floating 1280×720; the transition was not captured, and the normal
landscape observer remained active. No diagnostic geometry/focus dispatcher was
issued. These snapshots do not establish every earlier layout transition, but
the persistent extra process was observable before that later floating layout.
Do not attribute its creation or persistence to a measured observer resize.

### Expected helper lifecycle, unresolved hang

Source comparison shows that launching the game executable with the raw callback,
`runinprefix`, quiet output and detached `Start` predates the refactor. The landscape
change adds window-size/fullscreen arguments; it does not add a second callback
launch or change detached process lifetime. Installed UMU 1.4.4 routes namespace
re-entry through `steam-runtime-launch-client`; installed Proton's `runinprefix`
executes the supplied game command rather than forwarding it without a process.
The live callback process tree matches that route. Both live game processes have
`lsteamclient` disabled; that is not an observed difference explaining this hang.

Read-only static metadata/IL inspection of installed `TPCI.RainierClient.dll`
(SHA-256 `e7f20887ff93dcb2c4fdd48f0881e33eb7aa6632fb52d0503b086b9dcf8aaced`)
shows `EndPreload.UpdateDeepLinkSimulator` handing a supported callback to
`DeepLinkSimulator.SetDeepLink` / `UnityEngine.PlayerPrefs.SetString`, followed by
`UnityEngine.Application.Quit(0)`. Thus a helper launch is expected, but indefinite
survival is not its intended lifecycle. This is static control-flow evidence,
**not proof that this live process reached or returned from Quit**. No live
PlayerPrefs, registry credentials or authentication payloads were read, and no
proprietary files were changed. Quiet callback stdout/stderr are `/dev/null`;
there is no captured Unity callback log for this session.

A Unity/Wine helper-exit hang is the leading explanation, not a confirmed native
root cause or an established launcher fix. No hide, close, timeout, kill,
headless-mode change or replacement authentication transport was added. A next
targeted diagnostic is a native stack-only backtrace of the independently
identified helper, with arguments/locals disabled and identity rechecked. It
requires user consent to briefly pause that helper; no debugger was attached.

### Regression coverage, checks and preservation

Only tests and this validation record changed during this investigation; runtime
source was not changed. `internal/app/landscape_test.go` now models independent
owned primary/helper PIDs with the observed 1280×721 floating and 701×619 tiled
layouts, in either compositor order and across repeated observations. It asserts
only read-only client queries, with neither window mutated. The existing portrait
callback regression now also uses a distinct process instead of reusing the
primary PID. These tests do not simulate Unity shutdown or prove a duplicate fix.

`make fmt`, `make fmt-check`, `make lint`, `make check`, fresh
`go test -race -count=1 ./...`, `make check-shell`, all 11 `make test-plugin` tests,
read-only `make check-plugin`, and `git diff --check` passed. Formatting changed
only the edited test file. `make security` verified modules and reported zero
reachable/imported-package vulnerabilities, with one module-only advisory.
A separate release-style build succeeded at
`/tmp/opencode/ptcgl-black-window-build-2u06xv8l/ptcgl-omarchy`, SHA-256
`d6ca66c07acd8737cf51ebe16ef691e8bb723e1514096328c1c4efc538c46fb5`, version
`0.5.0-black-window-review`. It was neither launched nor installed and contains no
runtime fix. Private sanitized evidence/check logs are under
`/tmp/opencode/ptcgl-black-window-readonly-1vdesbet/`, with the later full snapshot
under `/tmp/opencode/ptcgl-black-window-readonly-10s4fx4n/`.

Both live game processes/windows were left open. Status remained ready/running,
maintenance disabled, with the active Play lock/session/operation safety guard
intact and no layout warning. The existing supervisor (PID 42292) remains active:
on normal Play exit it restores only its own MIME association to the original
installed handler and removes its uniquely named temporary desktop entry,
preserving concurrent preference edits. Restoration is still pending while this
session is open. The installed launcher, running candidate, game data, saved
desktop configuration, unrelated refactor work and user video were not replaced
or reset. No logout/login, shutdown, repair, cache clearing, commit or publication
was performed by this investigation. Duplicate-free fresh return and a completed
online match remain pending acceptance.

### Authorized backtrace deferred after identity recheck — 2026-10-30

The user authorized a brief, argument/local-free native backtrace of **only the
verified callback helper**, leaving the working game untouched. Before attaching,
the 08:07:39–08:08:49 UTC read-only checks found a changed session: original game
PID 43050 was absent, and no replacement game process/window was identified in the
owned prefix. PID 49441 retained its previous start identity, callback-scheme
marker, runtime executable, prefix ownership and mapped game/Unity modules, but
was now the **only** owned game window (floating 1280×720). Other owned Wine
processes had neither the game executable nor UnityPlayer mapped.

This proves callback provenance, not that the remaining window is still merely
a black helper rather than the user's current game. Because permission excluded
pausing the working game, no debugger attach or pause was attempted pending user
confirmation of the remaining window's role. No detach or resume signal was
needed; no process was closed, killed or restarted by the diagnostic. The reason
or exact time the original game exited is unknown; no crash has been established.

GDB 18.1 is installed, and the read-only Yama ptrace policy is 1. No permission
denial was tested, no privilege escalation or policy change was attempted, and
there is no new native stack evidence. Operation/status still reports running
with maintenance disabled, and the matching temporary callback handler,
supervisor and safety markers remain active. Sanitized identity/window/module
evidence is private under
`/tmp/opencode/ptcgl-black-window-readonly-tbva_fzm/`. Only this documentation was
changed for the deferred attempt; no runtime source or test changes were made.

### Authorized helper-only native trace — 2026-10-30

The user clarified: **“I have closed the game i cannot for some reason close the
hanging window.”** This establishes that the working primary was closed by the
user, not by the investigation, and identifies the remaining window as the hung
helper. Immediately before attachment, checks again required the same helper
start identity (PID 49441, start ticks 258639), UID, exact Proton Wine executable,
resolved prefix, callback-scheme marker and window address. Enumeration of owned
Wine processes with UnityPlayer mapped found no replacement game. The helper
had 62 threads, one running and 61 sleeping; none was stopped or traced.

Unprivileged GDB 18.1 attached at 08:14:57 UTC, captured all 62 native stacks and
detached normally. Debugger wall time was **0.206197 s**; the main thread's
observed stopped interval was approximately **0.111420 s**, sampled every 10 ms
(not an exact scheduler measurement). Stack collection itself took 0.021239 s.
All 62 tasks were checked after detach, not just the process leader: one was
running and 61 sleeping, all `TracerPid` values were zero, and none was stopped.
There was no timeout, ptrace denial, privilege escalation, emergency SIGCONT or
force-close. No other process was attached, paused or signalled.

The main native stack was:

```text
getrusage                         libc.so.6
NtYieldExecution                  ntdll.so
__wine_syscall_dispatcher         ntdll.so
server_init_process_done          ntdll.so
main                             wine
```

The 61 workers comprised 23 single-object fsync waits, 29
`NtWaitForAlertByThreadId` waits, two Wine window/message waits, one multiple-object
fsync wait, and six Mesa Radeon condition-variable waits. Together with sustained
near-one-core CPU usage on the main thread, this supports a busy yielding-loop
hang rather than a main thread blocked in a GPU wait or futex. It does **not**
establish which Unity operation is looping or prove a GPU/runtime root cause.
The native unwind did not expose the Windows/Unity caller across Wine's syscall
bridge; the older Unix loader frames are not evidence of a new startup stall.
The expected callback `Application.Quit(0)` path makes an exit/shutdown hang
plausible, but its exact live Windows call site remains unconfirmed.

GDB ran with `-q --nx --nh --batch`, frame arguments disabled, entry values
disabled, auto-loading/debuginfod disabled, and inferior calls, memory/register
writes and breakpoint/tracepoint insertion disabled. Only unwound PCs, sanitized
symbol/module/source basenames, thread IDs/states and timing were retained.
Unstructured debugger output was captured in memory and discarded, never written
or displayed. No arguments, locals, expressions, stack-memory dump, core,
credential data or Wine backtrace command was collected. The private filtered
trace and post-detach thread checks are under
`/tmp/opencode/ptcgl-helper-backtrace-__dngtve/`.

At 08:17:03 UTC the same black helper remained floating 1280×720, with all 62
threads untraced and unstopped. The original primary remained absent. The active
Play lock, operation/session markers, prearmed safety guard and matching temporary
callback association/supervisor remained intact; status still reports running
with maintenance disabled. The persistent helper is not grounds to delete or
bypass those markers. The installed launcher and running candidate were unchanged.

No safe launcher-level correction follows from this stack alone, so no runtime
source/test change, build or live fix was applied. Only this validation record
changed for the trace; `git diff --check` passed. The existing regression tests
and their earlier automated results are unchanged. The next recovery option,
**only with explicit additional consent**, is a PID-bound force termination of
this reverified hung helper alone, never its process group, wineserver or launcher.
Before any such action, recheck that no working game has started; afterward verify
the entire helper task set has exited, allow normal Play/supervisor cleanup and
verify callback restoration without altering safety guards. No force termination
has been performed. Duplicate-free fresh login and online-match acceptance remain
unresolved.

### Authorized helper-only termination and normal cleanup — 2026-10-30

After the trace, the user explicitly approved force-closing only the hung
callback helper. Fresh checks again found exactly its known game window and no
replacement game. UID, exact Proton Wine executable, resolved prefix, game-path
marker, callback provenance, PID **49441**, start ticks **258639**, and address
`0x55dc030952c0` matched the prior capture. The candidate Play process and temporary
supervisor still owned the session, and all 62 helper tasks were untraced.

At **08:24:56.701 UTC**, one `SIGKILL` was sent with `pidfd_send_signal` through a
fresh helper pidfd, with identity and no-new-game checks repeated after opening
the descriptor. No process group, wineserver, launcher or unrelated process was
signalled, and no game restart was issued. This was a consented recovery action,
not a launcher policy or an authentication fix.

Task-aware observation initially saw a zombie leader **and one still-running
worker**, so that snapshot was explicitly not counted as termination. By the
next observation, approximately 0.202 s after the signal, the pidfd indicated
exit, the original process identity was absent and no helper tasks remained.
The black window disappeared. The remaining runtime/session then exited through
normal launcher lifecycle cleanup without further signals: Play and its
supervisor were gone, the session flock was released, and the play-session,
operation and prearmed cleanup-guard markers were absent by approximately
0.603 s. No marker was manually deleted or bypassed.

The supervisor recorded normal Play exit code **0** at 08:24:57.262 UTC and restored
`ptcgl-omarchy-callback.desktop` itself, then removed its temporary desktop entry.
The MIME preferences match the original pre-test bytes exactly. Its final
preservation event confirms that both installed launcher and installed callback
entry remained unchanged; installed/candidate hashes still match the earlier
record. No callback association was manually changed during recovery.

Post-cleanup status is **ready / idle**, maintenance is enabled, no owned game
window or prefix process remains, and no previously tracked runtime/session
process has live tasks. There is no remaining runtime child holding the Play
lock and no dangling temporary callback route. No further termination or cleanup
consent is needed for this completed recovery. Private sanitized signal,
task-transition and restoration evidence is under
`/tmp/opencode/ptcgl-helper-close-5r770f_g/`.

Only this validation record changed for recovery; `git diff --check` passed.
No runtime source change, automatic callback-kill workaround, build, installation,
repair, cache clearing, new launch, commit or publication was performed. Existing
uncommitted source work and the user video were preserved. The callback hang's
exact Windows/Unity cause and a supported source fix remain unresolved; terminating
the helper does not pass duplicate-free fresh-login or online-match acceptance.

### Bounded callback helper cleanup — live accepted

The launcher now supervises the callback helper instead of treating it as a
fire-and-forget process. `Callback` runs the helper through the existing
`Runner.Run` process-group path with a 30-second default bound. Landscape
arguments, the restricted environment, `runinprefix`, quiet output and the
no-logging policy are unchanged. If the helper has not exited by the deadline,
the normal cleanup sends `SIGTERM`, escalates to `SIGKILL` after the existing
grace period, and verifies all threads are gone. A confirmed timeout is treated
as a completed handoff because static client inspection shows the helper writes
the deep link and calls `Application.Quit(0)`; the helper is always secondary
because `Callback` requires an active Play session. An unconfirmed cleanup is
reported without exposing callback arguments. A helper that exits with a
non-zero status after running is not treated as an authentication failure; the
game remains the authority on whether the callback was accepted.

This is a lifecycle mitigation for the observed Unity/Wine shutdown hang, not a
native root-cause fix. Regression coverage asserts supervised `Run` rather than
detached `Start`, the deadline, quiet/landscape/reduced environment,
timeout-as-handoff, non-zero helper exit, cancellation before start, cleanup
failure without secret leakage, and a real TERM-ignoring helper whose process
group is killed and verified absent.

A separate candidate (`0.5.0-callback-cleanup`, SHA-256
`5af25a1d7d775559b2d11bac66c2e6f679038479271cf8302d289741944b4c42`) passed
ready/idle preflight and all 15 doctor checks. The normal candidate Play started
at 08:46:33 UTC with no diagnostic overlay or external geometry helper. The game
window appeared at 1416×1248 and the landscape observer corrected it to
1280×720 floating when needed. The user completed a fresh browser logout/login,
reached the game, and confirmed an online match completed and returned to the
menu.

The callback helper window appeared at 08:48:00 UTC and disappeared by 08:48:04
UTC, approximately 3.5 seconds later; the user confirmed it closed by itself
right after login. No second black window remained. Because the helper exited
before the 30-second deadline, the live run did not exercise the timeout kill;
that path remains covered by the real TERM-ignoring-helper regression test. The
user then closed Play normally at 08:53:54 UTC. Play exited 0, the supervisor
restored `ptcgl-omarchy-callback.desktop`, and the session lock, cleanup guard,
play-session marker and operation marker were all absent. No game window or
owned game process remained. The installed `0.5.0` launcher and its SHA-256
`eb6b817b5fce8684b9ba34badf76cf104179b95551ca5544d1204501f6f43d30` were
unchanged; the candidate was not installed.

`make fmt-check`, `make lint`, `make check`, fresh
`go test -race -count=1 ./...`, `make security`, and `git diff --check` passed.
Security reports zero reachable/imported-package vulnerabilities and one
module-only advisory. No installed launcher, saved window rule or game data was
changed.
