# Game and runtime updates

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
- Installation receipts are advanced only after Windows Installer succeeds
  and the game executable exists.
- A failed MSI invocation can partially change installer-managed files. Run
  `repair` after an interrupted installation. Automatic rollback of Windows
  Installer changes is not provided by the launcher.
- One advisory session lock serializes setup, game lifetime, update, repair,
  integration, and runtime changes. Login callbacks bypass this lock only for
  an active game session.

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
