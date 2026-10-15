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
