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

Window startup and integration were validated. An online match and a future
changed-MSI upgrade have not been recorded in this validation session. The
automated suite covers changed installer application, same-MSI no-op updates,
explicit repair, preservation of registry/settings data, and failed updates.
