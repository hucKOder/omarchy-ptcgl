# Implementation plan

## Goal

A standalone Go repository for installing and launching Pokémon TCG Live on
Omarchy, with app-menu launch, browser authentication, and a tiled Hyprland
window. Use the official Windows client through UMU, GE-Proton, and XWayland.

## Current status and next work

### UX milestones 1–3 — v0.5.0 stable promotion, 2026-10-29

The maintainer approved **v0.5.0** stable promotion after successful installed
v0.5.0-rc.1 logout/browser-login and gameplay testing. The RC's hosted release/rebuild,
downloaded-artifact authentication, and isolated launcher-only installation passed;
repeat those artifact gates for the stable tag. The earlier temporary deployment
was restored, then the published RC and matching companion were installed for the
user's test. See the [live acceptance record](VALIDATION.md).

- [x] Milestone 1: schema-1 local status API with validated lock ownership, stale
  marker rejection, startup feedback, repeat-launch waiting, compact expandable
  companion sections, active-session maintenance gating, and protocol negotiation.
- [x] Milestone 2: known/unknown-size transfer progress and speed, download-only
  cancellation, staged managed-runtime recovery, pending MSI detection, and receipt-
  based readiness. Prefix/settings, verified cache, and prior successful receipts
  survive failures; partial HTTP resume and MSI rollback are not implemented.
- [x] Milestone 3: confirmed allowlisted recovery, privacy-safe saved/copied reports,
  and an explicit component update overview with dated installer comparisons.
- [x] Existing-installation desktop smoke test: real app-menu startup, repeat
  launch/focus, download cancellation and MSI verification, cancelled repair,
  diagnostics/GUI report saving, fullscreen/resize/workspace behavior, both
  monitors, and game survival through shell restart. Original installation restored.
- [x] Publish the opt-in v0.5.0-rc.1 candidate through protected main; verify exact
  hosted/local rebuilds, all artifact attestations, pinned bootstrap installation,
  isolated launcher-only reinstall preservation, and read-only official MSI verification.
- [x] Installed RC acceptance: authenticated launcher upgrade and matching companion,
  all 15 diagnostics, state/integration preservation, and user-confirmed logout/login
  and gameplay. Stable promotion approved; no further runtime behavior changes.
- [ ] Remaining desktop validation: vertical bar, slow/failing startup variants,
  actual interrupted MSI/runtime recovery,
  changed-MSI application, and clipboard-copy behavior on the live desktop.
- [ ] Clean-OS first-time tester. The maintainer accepted this remaining validation
  gap for stable promotion; it and the desktop follow-ups above are not marked passed.
  Companion marketplace submission remains separate. Preserve release security gates.
- [ ] Publish v0.5.0 and verify its own artifacts, attestations, exact rebuild, and
  authenticated isolated launcher-only installation before announcing completion.

See [companion controls and status protocol](PLUGIN.md) and
[maintenance/recovery behavior](UPDATES.md).

Build steps 1–4 below are implemented. Real-machine setup, browser callback
delivery, tiled startup, resizing, and repeat-launch focus are recorded in
[the validation record](VALIDATION.md).

- [x] Actionable diagnostics: `doctor` explains failures and suggests recovery
  commands in terminal/GUI reports and an optional `fix` field in JSON. Check
  the callback desktop file as well as its MIME registration, and preserve
  command errors. Automated regression coverage added on 2026-10-15.
- [x] Online-play acceptance: the user confirmed multiple completed online
  sessions without issues on 2026-10-15; see the validation record.
- [x] Recorded desktop acceptance on 2026-10-15: workspace movement and the
  compositor fullscreen operation used by Omarchy's binding, alongside resize,
  repeat launch, maintenance blocking, and a successful post-repair launch.
- [ ] Record a real changed-MSI upgrade when a newer official installer is
  available, including preservation of existing settings.
- [x] Smoke-tested v0.3.0 published downloads and attestations, then completed a
  launcher-only upgrade using the published installation block. The installed
  binary matched the published digest and all 16 diagnostics passed.

## Security priority — 2026-10-22

The [security audit and current-incident comparison](SECURITY-AUDIT.md) takes
priority over installation QoL and companion marketplace publication. It relates
the PHP2Shell, GitLab, eMudhra, and LiteLLM incidents to this project's actual
input, certificate, credential, and release boundaries. The PHP/GitLab CVEs do
not directly apply; certificate trust and supply-chain compromise do.

During active gameplay, use source-only work, developer tooling, read-only
queries, and isolated fixtures. Schedule deployment and game-touching acceptance
after the current session exits.

- [x] Review current source, published artifact provenance, and repository release
  controls. Reproduce revoked-intermediate acceptance, transitive archive-link
  escape, and synthetic runtime credential inheritance in isolated fixtures.
- [x] Enable and verify Go/Bash LSP diagnostics in OpenCode, with user-local Bash
  LSP and ShellCheck tooling. Record existing shell diagnostics and the upstream
  developer-only dependency advisory in the audit.
- [x] **P0 source — SEC-02:** require Go 1.27.3+ in `go.mod`, build a separate
  security candidate, and verify compromised-key rejection through the actual
  downloader TLS path against both original-root and cross-signed test hosts.
  A new published/installed binary remains part of release acceptance.
- [x] Separately assess `curl`/`gh` trust paths and installation-host trust-store
  updates. Both initially accepted the original-root compromised-CA test host;
  the signed Arch CA update and patched active GitHub CLI now reject both hosts.
  The initial HTTPS source bootstrap remains an explicit trust boundary.
- [x] **P0 source — SEC-01:** check revocation across every non-root certificate in an
  accepted publisher/timestamp chain. Add regressions for revoked intermediates,
  missing/unavailable evidence, alternate chains, and valid timestamped installers.
  The candidate also passed read-only verification of the cached official MSI.
- [x] **P1 source — SEC-04:** prepare CODEOWNERS, a fail-closed ruleset/environment
  policy check, reviewed-main ancestry checking, and a separate-runner byte-for-byte
  rebuild gate. Negative fixtures cover weak/missing protection settings.
- [x] **P1 settings activation — SEC-04:** protect `main`, release tags, and publishing
  with the documented solo-maintainer policy, CI-bound required checks, no bypass,
  and an owner-approved tag-only `release` environment. The real policy check passes.
- [x] Validate hosted public-metadata checks and the separate-runner rebuild in
  GitHub. The full owner-authenticated no-bypass audit passed before each release
  approval; the solo-maintainer process does not provide independent human review.
- [x] **P1 source — SEC-05:** constrain game/runtime and MSI-worker environments; verify that
  synthetic GitHub/cloud/model credentials are excluded from play, setup,
  maintenance, and callbacks while required desktop/runtime settings survive.
  Document that a Wine prefix does not isolate same-user credential files.
- [x] **P1 source — SEC-03:** confine archive operations with `os.Root`, validate the
  complete link graph before link creation, and retain checksum pinning. Cover
  transitive and dangling escapes, cycles, valid relative chains, and dangling links.
- [x] **P1 source:** schedule daily read-only vulnerability checks using the declared
  build toolchain and document compromised-but-authentic release recovery. Existing
  installer fixtures retain source/workflow/repository mismatch rejection.
- [x] Fix the existing ShellCheck warnings and add `make check-shell` to CI/releases.
- [ ] **P2:** investigate verifier memory budgets and malformed-input lifecycle
  fuzzing; track the developer-only Bash LSP `braces` advisory and filesystem/network
  sandboxing separately from environment filtering.
- [x] Validate the installed candidate's real MSI verification, upgrade preservation,
  fresh isolated runtime/game provisioning, and automated desktop behavior.
- [x] Recheck fresh browser login and gameplay after the reported 61/61 login stall.
  The user confirmed a clean fresh login and completed match with graphics/audio;
  the initial failure/recovery remains recorded without a confirmed launcher cause.
- [x] Publish v0.4.1 with verified provenance and exact hosted/local build comparison;
  authenticate and install the published upgrade with preservation checks.

## Installation follow-up

After the security priority above, installation simplification takes priority
over the remaining launcher QoL work.
See [the installation plan](INSTALLATION-PLAN.md) for dependencies, implementation
tasks, and release acceptance criteria:

1. Replace the external `osslsigncode` dependency with built-in verification.
2. Generate one complete release-specific installation procedure, with no manual
   version/commit transcription or apparent second installation step.
3. Improve prerequisite checks and failure messages.
4. Add understandable setup progress and reliable, stage-specific recovery.
5. Validate the published procedure on clean Omarchy with a first-time tester.

The v0.4.0 release embeds pinned Relic MSI verification, eliminating verifier
packaging and AUR provisioning. Its source bootstrap installs repository tools,
selects a release, and authenticates the release installer before execution.
The README now leads with one command. Published assets and the launcher-only
one-command flow passed isolated HOME/XDG verification; clean-OS acceptance
remains a separate task. See [the v0.4.0 validation record](VALIDATION.md#published-v040-verification--2026-10-15).

Release-specific instruction generation, ready-to-run v0.2.0 instructions, and
early GitHub CLI capability checks are implemented and covered by isolated tests.
The earlier package investigation confirmed `osslsigncode` was absent from the
checked Arch/Omarchy binary channels. That route is retained only for legacy
releases; setup progress/recovery and clean-OS acceptance remain open.
Published v0.3.0 authentication and the
launcher-only upgrade are verified in the validation record.

The local candidate was independently reviewed, review findings were fixed, and
real desktop/maintenance checks passed on 2026-10-15. See the validation record
for the tested development version, binary hash, and acceptance scope.

A subsequent isolated fresh-app installation also passed runtime/prefix/game
provisioning and first browser sign-in. The initial cross-profile callback-routing
mistake was corrected in the test harness, and the normal installation was
restored and checksum-verified afterward. Clean-OS dependency provisioning is
still a separate acceptance task.

## QoL roadmap

Prioritized on 2026-10-15, following successful online-play acceptance. Effort
estimates are relative: **small** is a focused change using existing operations;
**medium** involves coordinating UI and process/download state.

### 1. App-menu maintenance actions and clear results — completed 2026-10-15

- [x] Add **Check installer updates** and **Repair game** desktop actions, using
  the existing `update --check --gui` and `repair --gui` commands.
- Show a result dialog after graphical maintenance: installer unchanged, newer
  installer available, update applied, or repair completed. Game patches continue
  to be checked by the game at launch.
- Use operation-specific titles and progress text for checking, updating, and
  repairing.
- Completion: each outcome is understandable without a terminal; failures and
  running-session conflicts remain visible; integration refreshes the actions
  consistently. Verify changed/unchanged installer outcomes and GUI result routing.
- Verified with isolated installer/Zenity fixtures for result dialogs, failures,
  busy sessions, and repeated desktop integration.

### 2. Real download progress — implemented in checkout; desktop acceptance pending

Included in the installation plan's setup-progress work.

- [x] Show downloaded/total MiB, percentage, and transfer speed for MSI and Proton
  downloads. Use indeterminate progress when the server omits the total size.
- Show distinct downloading, verifying, extracting, and installing phases.
- Allow cancellation during downloads, with temporary-file cleanup and a usable
  retry. Cancellation availability follows the operation phase.
- Completion: known/unknown-length downloads and cancellation report accurately;
  interrupted transfers preserve the existing verified cache.

### 3. Smoother startup and repeat launch — implemented in checkout; desktop acceptance pending

- [x] Show brief **Starting Pokémon TCG Live…** feedback until the owned game
  window appears, then dismiss it without taking focus from the game.
- A second launch during startup waits briefly for the same session's window and
  focuses it. Report setup/maintenance contention separately from game startup.
- Completion: slow startup and rapid repeated launches produce one game session,
  useful feedback, and correct focus; failed startup clears the indicator.

### 4. Open game and configuration folders — completed 2026-10-15

- [x] Add a folder-opening command and an app-menu **Open folders** chooser for
  the game installation, launcher configuration, and download cache.
- Resolve paths from the active XDG configuration and explain missing folders.
- Completion: folders open in the default file manager, including paths with
  spaces or Unicode and custom XDG locations.
- Verified with isolated chooser/opener fixtures, including cancellation,
  missing folders, custom XDG paths, active sessions, and broken configuration.

### 5. Shareable diagnostics summary — implemented in checkout; desktop acceptance pending

- [x] Add launcher/runtime versions and the last applied installer date to the
  diagnostics report, plus a **Save report** action in the graphical view.
- Completion: one readable report contains version/context details and actionable
  checks; saving or cancelling the save dialog behaves predictably.

## Build sequence

1. Implement the CLI, XDG paths, isolated prefix, runtime provisioning, official
   MSI installation, and game process launch.
2. Implement game updates: preserve the prefix; run the built-in updater with
   the correct working directory; provide fresh MSI checks, installation,
   repair, atomic downloads, and failure recovery. Block maintenance while
   the game is running. Keep game, launcher, and Proton updates separate.
3. Implement browser URI handling in the running game's environment, desktop
   entries, narrowly scoped Lua window rules, diagnostics, and uninstall.
4. Add build/install targets, CI, documentation, and tests of failure paths.
5. Validate real installation, game startup, authentication, update checks,
   tiling, resizing, workspace changes, repeat launch, and gameplay on Omarchy.

## Automated acceptance checks

- `make check` passes formatting, vet, and unit tests.
- `make test` passes the race-enabled test suite.
- `make build` produces a standalone Linux executable.
- Bad/interrupted downloads preserve the existing cache.
- Invalid archives and non-Pokémon installer publishers are rejected.
- Update and repair retain registry/settings data and serialize with gameplay.
- Same-MSI update is a no-op; explicit repair restores installer-managed files.
- Callback delivery uses the active session and suppresses secret-bearing output.
- Repeated integration is idempotent; invalid Hyprland rules roll back.
- A launcher upgrade retains the previously selected runtime profile.
- Security regression tests reject revoked publisher/timestamp intermediates and
  transitively escaping archive links, and constrain inherited runtime secrets.
- Source-aware vulnerability analysis passes with the release build's supported,
  patched Go toolchain. Published artifacts identify that toolchain and match
  reviewed source and an independent clean build.

## Real-system acceptance checks

- Fresh prefix initialization, Visual C++ runtime installation, and official MSI
  installation complete.
- Hyprland reload and `configerrors` succeed after integration.
- The app-menu entry launches the game; its window is tiled and not fullscreen.
- Browser login returns to the same game process.
- Resize, workspace movement, and Omarchy fullscreen shortcuts work.
- Repeat launch focuses the existing window.
- `update --check` authenticates the current official installer; an unchanged
  installer does not trigger a reinstall.
- The game's own updater completes and a match can be played.
