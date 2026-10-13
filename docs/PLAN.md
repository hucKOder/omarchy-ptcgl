# Implementation plan

## Goal

A standalone Go repository for installing and launching Pokémon TCG Live on
Omarchy, with app-menu launch, browser authentication, and a tiled Hyprland
window. Use the official Windows client through UMU, GE-Proton, and XWayland.

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
