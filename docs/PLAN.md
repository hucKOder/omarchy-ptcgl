# Implementation plan

## Goal

A standalone Go repository for installing and launching Pokémon TCG Live on
Omarchy, with app-menu launch, browser authentication, and a tiled Hyprland
window. Use the official Windows client through UMU, GE-Proton, and XWayland.

## Current status and next work

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

## Installation follow-up

Installation simplification takes priority over the remaining launcher QoL work.
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

### 2. Real download progress — medium

Included in the installation plan's setup-progress work.

- [ ] Show downloaded/total MiB, percentage, and transfer speed for MSI and Proton
  downloads. Use indeterminate progress when the server omits the total size.
- Show distinct downloading, verifying, extracting, and installing phases.
- Allow cancellation during downloads, with temporary-file cleanup and a usable
  retry. Cancellation availability follows the operation phase.
- Completion: known/unknown-length downloads and cancellation report accurately;
  interrupted transfers preserve the existing verified cache.

### 3. Smoother startup and repeat launch — medium

- [ ] Show brief **Starting Pokémon TCG Live…** feedback until the owned game
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

### 5. Shareable diagnostics summary — small

- [ ] Add launcher/runtime versions and the last applied installer date to the
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
