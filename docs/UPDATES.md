# Game and runtime updates

## v0.5.0 maintenance experience

The v0.5.0 release adds `updates [--gui]`, `status [--json]`, confirmed `fix --gui`
actions, measured download progress, and support report export. These changes are
not in the older v0.4.1 launcher. Existing-installation desktop smoke testing and
user-confirmed candidate logout/login and gameplay passed. Real MSI repair,
interrupted-install recovery, changed-installer application, and clean-OS coverage
remain validation follow-ups. See [the live validation record](VALIDATION.md).
The desktop entry adds **Versions & updates** and routes **Repair game** through
the confirmed recovery flow. Existing terminal `repair --gui` remains available.

`updates --gui` brings component information together without coupling updates:
launcher upgrades still use authenticated release installation, installer actions
only compare/apply the official MSI, and runtime selection is separately confirmed.
Custom runtime overrides remain user-managed. No component is upgraded automatically.
Installer comparisons show their check timestamp, never a general “game up to date”
claim. Normal patches remain the game's responsibility.

## Normal game patches

`play` launches the game's built-in updater and client from the directory
containing `Pokemon TCG Live.exe`. The working directory matters: the updater
uses relative paths. Windows game files, registry state, settings, and login
state live in the application's existing Proton prefix.

The game owns patch discovery and delivery. No independent patch feed, version
API, or updater implementation is assumed by this project.

## Official MSI refresh

The optional `update --check` downloads the current official MSI, verifies its
Authenticode signature and publisher, and compares its SHA-256 with the
installation receipt.
It does not invoke Windows Installer or check in-game patches.

You can run `update` directly; a separate check is not required. It performs the
same download and verification. If the MSI changed, it
invokes Windows Installer in the existing prefix. The official installer owns
its upgrade behavior. If the MSI has not changed, no reinstall is performed;
launching the game can still apply newer in-game patches.

`repair` downloads the current installer and reapplies it. For the exact MSI
recorded as last applied, repair uses `REINSTALL=ALL REINSTALLMODE=vomus` to
restore installer-managed files. For a changed MSI, it uses the installer's
ordinary installation/upgrade path. A missing game can also be installed with
`repair`.

These commands never recreate or delete the prefix. They do not guarantee that
Pokémon's own installer or game will preserve every preference across all
future releases.

### Graphical maintenance

The expanded desktop actions and result dialogs in this section require v0.3.0
or later. The v0.2.0 release supports the CLI maintenance commands described
above.

The application's desktop actions provide:

| Action | Command | Successful result |
| --- | --- | --- |
| Check installer updates | `update --check --gui` | Installer unchanged, or an installer update is available |
| Update game | `update --gui` | Installer unchanged, or update completed |
| Repair game | `repair --gui` | Repair completed |

Each command shows an operation-specific progress dialog followed by its result.
An available installer is applied using **Update game**. In-game patches are
checked when the game launches. Failures and running-session conflicts produce
an error dialog. Close the game before using these maintenance actions.

## Interrupted operations

- Downloads are written to private temporary files. An incomplete, oversized,
  or invalid download is never published over a previously cached installer.
- Download failures can be retried with the same command. A retry restarts the
  download; partial HTTP resume is deliberately not implemented.
- GE-Proton archives are checked against a pinned SHA-256 and extracted into a
  staging directory before the completed runtime is published.
- In v0.5.0+, an incomplete **managed** runtime is replaced only
  after a validated replacement is staged. Failed extraction leaves the original
  untouched. Custom `proton_path` locations are never automatically repaired.
- The security candidate confines extraction to a fresh filesystem root and
  checks complete relative-symlink chains, including dangling targets and cycles.
- Installation receipts are advanced only after Windows Installer succeeds
  and the game executable exists.
- v0.5.0+ writes `installer-pending.json` before MSI application
  and removes it only after success is recorded. A leftover executable is not
  sufficient: a valid successful receipt is required for “Ready.” Interrupted or
  unrecorded installations produce a repair action; retrying `setup` reapplies the
  installer in the existing prefix instead of declaring completion.
- A failed MSI invocation can partially change installer-managed files. Run
  `repair` after an interrupted installation. Automatic rollback of Windows
  Installer changes is not provided by the launcher.
- One advisory session lock serializes setup, game lifetime, update, repair,
  integration, and runtime changes. Login callbacks bypass this lock only for
  an active game session.
- The v0.5.0+ GUI shows downloading/verifying/extracting/installing phases.
  Download cancellation removes temporary files and preserves verified cache.
  Cancellation through the dialog cannot terminate verification or Windows
  Installer. Closing startup feedback or the companion never cancels the game.

For guided recovery, open Diagnostics and choose a supported fix. Each graphical
fix explains the change and asks for confirmation. `fix` accepts only fixed action
IDs (`integration`, `setup`, `repair`, `runtime-update`); diagnostic `fix` strings
are never executed. Diagnostics offers **Save report** and optional **Copy report**;
the exported report excludes personal paths and raw subprocess/error output.

## Launcher updates

Close the game and choose the intended release. Use its complete installation
block from the release notes, appending `--launcher-only` to the final `bash`
command inside the block. The published version and source commit are already
selected. For v0.2.0, use the
[ready-to-run verified procedure](RELEASES.md#verified-release-installation).

It installs only the explicitly selected release. The installer verifies the
manifest's attestation against the expected repository, workflow, tag, source
commit, and GitHub-hosted runner policy, then checks the launcher hash. A missing
release or any verification failure stops before package installation or launcher
replacement. There is no automatic build-from-`main` fallback.

For a local development checkout, obtain the new repository version, then run:

```sh
make check
make install-launcher
```

The executable is replaced atomically and desktop integration is refreshed.
The application's game prefix and previously selected runtime are retained.

### Security-candidate behavior

The 2026-10-22 bulk patch requires Go 1.27.3+, checks current revocation evidence
for every non-root publisher/timestamp certificate, and rejects a chain with
missing CRL distribution points. A valid alternate chain can still succeed. The
candidate passed installed-launcher preservation, official MSI verification, fresh
isolated provisioning, and user-confirmed fresh login/gameplay acceptance. The
validation record also retains an initial 61/61 login stall and recovery without
claiming a confirmed launcher cause or authentication-code fix.

Game, setup, maintenance, callback, and MSI-verifier subprocesses inherit an
explicit environment allowlist rather than every terminal variable. Display,
XDG paths, locale, audio, selected graphics/runtime settings, proxy settings,
and certificate-store paths are retained. Unknown variables, developer/cloud/model
credentials, `LD_PRELOAD`, and `LD_LIBRARY_PATH` are not forwarded. Callback log
suppression remains explicit. Review custom graphics overlays and unusual runtime
overrides during closed-game acceptance, because unlisted overrides are dropped.

This filtering is not a sandbox. The runtime and game still have the normal
user's filesystem access; required proxy/display settings can themselves carry
sensitive connection information. A dedicated Wine prefix separates game state,
not operating-system permissions.

Build/test work can proceed during gameplay. Apply the launcher update and run
integration, runtime, browser, and game smoke tests after the active session ends.

## Compatibility-runtime updates and rollback

A launcher release includes a tested compatibility profile in
`internal/app/assets/compatibility.json`. The installed selection is saved in
`~/.local/state/ptcgl-omarchy/runtime.json`.

To explicitly select the newly bundled profile:

```sh
ptcgl-omarchy runtime-update
```

The new runtime is downloaded and validated before the selection changes.
Previous runtime directories remain installed. To select an older runtime
for troubleshooting, set its absolute directory in `config.json`:

```json
{
  "proton_path": "/home/you/.local/share/ptcgl-omarchy/runtime/GE-Proton11-7-x86_64"
}
```

Remove that override to return to the managed runtime. Switching Proton can
upgrade the Wine prefix when the game next starts; a compatibility-runtime
rollback is not a rollback of Windows registry changes.

UMU's Steam Linux Runtime is managed and updated by UMU. This is distinct from
the GE-Proton version selected by this launcher. System UMU updates are handled
through Omarchy's package updates.
