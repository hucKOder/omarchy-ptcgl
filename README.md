<p align="center">
  <img src="docs/assets/banner.svg" alt="Omarchy PTCGL in bold block ASCII lettering" width="100%">
</p>

<p align="center">
  <strong>Pokémon TCG Live. Tiled, resized, and right at home on Omarchy.</strong>
</p>

<p align="center">
  <a href="https://github.com/hucKOder/omarchy-ptcgl/actions/workflows/ci.yml"><img src="https://github.com/hucKOder/omarchy-ptcgl/actions/workflows/ci.yml/badge.svg" alt="Go checks"></a>
  <a href="https://omarchy.org"><img src="https://img.shields.io/badge/made_for-Omarchy-a78bfa?style=flat-square&amp;labelColor=1b1925" alt="Made for Omarchy"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-f7d37c?style=flat-square&amp;labelColor=1b1925" alt="MIT license"></a>
</p>

<p align="center">
  <a href="#install">Install</a> ·
  <a href="#omarchy-companion">Companion</a> ·
  <a href="#demo">Watch the demo</a> ·
  <a href="#updates">Updates</a> ·
  <a href="#development">Development</a>
</p>

## A proper desktop window

Play the official Pokémon TCG Live client alongside your editor, terminal, and
everything else. Launch from Omarchy's app menu, resize its tile, move between
workspaces, and use your usual window shortcuts.

**Omarchy PTCGL** is a small Go installer and launcher. It runs the Windows game
through **UMU + GE-Proton + XWayland**, with the desktop details handled for you:

- **Tiled from the start.** Windowed startup and game-specific Hyprland rules
  handle Wine's fixed-size hints. `Super + F` still toggles compositor fullscreen.
- **Launch once, focus again.** Reopening the app focuses the existing game.
- **Sign in through your browser.** The login callback returns to the running game.
- **Keep your own game environment.** A dedicated prefix and pinned Proton build.
- **Update on your terms.** Game patches, installer updates, and Proton upgrades
  have separate paths.

After first-time setup, the app-menu entry opens straight into the game.

## Demo

https://github.com/user-attachments/assets/5066d5b3-5bf4-4150-998e-15bc940236e6

*Recorded on Omarchy: Pokémon TCG Live sharing the desktop with an editor and
terminals while the layout changes.*

[Open the recording](https://github.com/hucKOder/omarchy-ptcgl/raw/refs/heads/main/docs/assets/tiling-demo.mp4)
· [View a still frame](docs/assets/tiling-demo.webp)

## Install

Use an updated system trust store before the download command. For the October
2026 eMudhra incident, Arch's fixed CA package is `ca-certificates-mozilla`
`3.131.1-1` or newer. GitHub CLI `2.102.1` built with Go `1.27.3` is tested.
The initial `curl` download and GitHub CLI use their own TLS verification paths;
updating the launcher alone does not update those tools.

On **x86-64 Omarchy** with Lua Hyprland configuration (0.55+), working
XWayland/Vulkan, and **8 GB free**, paste this into a terminal as your normal user:

```bash
bash -c 'script=$(curl -fsSL https://raw.githubusercontent.com/hucKOder/omarchy-ptcgl/main/bootstrap.sh) && bash -c "$script"'
```

The command installs dependencies, verifies the launcher release, sets up Proton
and the game, and adds **Pokémon TCG Live (Omarchy)** to the app menu. Enter your
sudo password when Omarchy asks, then open the app and sign in. The first download
can take a while. Close the game before installing or upgrading.

**No AUR packages required.** Game-installer signature and publisher verification
is built into the launcher. [Installation and verification details](docs/RELEASES.md).

### Play

Open **Pokémon TCG Live (Omarchy)** from the app menu, or run:

```bash
ptcgl-omarchy
```

Sign in using your normal browser when the game asks. If a download or runtime
setup is interrupted after the launcher is installed, run `ptcgl-omarchy setup`
to retry. If Windows Installer failed or was interrupted, run
`ptcgl-omarchy repair`. See [the recovery guide](docs/UPDATES.md#interrupted-operations).

<details>
<summary><strong>Install only the launcher, or install from a checkout</strong></summary>

To leave the game download for the first app-menu launch:

```bash
bash -c 'script=$(curl -fsSL https://raw.githubusercontent.com/hucKOder/omarchy-ptcgl/main/bootstrap.sh) && bash -c "$script" -- --launcher-only'
```

For an independently pinned version and source commit, use the complete
installation block in the [selected release's notes](https://github.com/hucKOder/omarchy-ptcgl/releases).

To build and install a local checkout, including dependencies and game setup:

```bash
git clone https://github.com/hucKOder/omarchy-ptcgl.git
cd omarchy-ptcgl
git checkout --detach FULL_40_HEX_COMMIT
# Review the selected source before executing it.
bash ./install.sh --local
```

Replace `FULL_40_HEX_COMMIT` with the source revision you intend to build.
`make install` runs the explicit local installation flow. Local builds need Go 1.27.3+
and Make; the installer adds them through Omarchy when needed.

Prefer the terminal? Setup and launch are also available separately:

```bash
ptcgl-omarchy setup
ptcgl-omarchy play
```

</details>

### Open folders

The folder shortcuts, expanded diagnostic fixes, and maintenance result dialogs
described below require v0.3.0 or later.

Use the **Open folders** desktop action to choose the game installation,
launcher configuration, or download cache, or open a folder directly:

```bash
ptcgl-omarchy open game
ptcgl-omarchy open config
ptcgl-omarchy open cache
ptcgl-omarchy open --gui
```

Folders open in your default file manager using the active XDG paths. These
shortcuts work while the game is running and even when launcher configuration
needs fixing. A missing folder is explained with its location; the configuration
folder is optional until you add `config.json`.

## Omarchy companion

**PTCGL Companion** adds a native Omarchy Quattro bar button and themed panel for
Play/focus, first-time installation, game maintenance, diagnostics, and folders.
It uses the same launcher and graphical dialogs as the desktop app.

The companion requires **launcher v0.5.0 or later** (live status schema 1).
Older launchers produce an upgrade prompt. The panel's install/upgrade action
uses the same authenticated stable-release installation as the normal bootstrap;
launcher, companion, game, and Proton updates remain separate.

To validate, copy, and enable the companion from a local checkout on Omarchy:

```bash
make install-plugin
```

This needs Qt's `qmllint` and refuses to overwrite an existing plugin.
Alternatively, install the published companion source through Omarchy with:

```bash
omarchy plugin add https://github.com/hucKOder/omarchy-ptcgl.git --enable
```

Click **PTCGL** in the bar. If the launcher is missing, **Install launcher** opens
a terminal for the existing authenticated release installation. Reopen the panel
and choose **Set up & play** afterward. Already-installed launchers are detected
automatically. Removing the companion retains the launcher and game:

```bash
omarchy plugin remove io.github.huckoder.ptcgl
```

[Companion controls, dependencies, updates, removal, and development](docs/PLUGIN.md).
Marketplace publication is pending.

## Updates

**Game patches happen at launch.** The client runs from its installation
directory so its built-in updater can apply patches correctly.

**Official installer updates are explicit.** Close the game, then run:

```bash
ptcgl-omarchy update
```

An unchanged MSI is skipped. Updates and repairs use the existing prefix;
maintenance is blocked while the launcher-managed game session is running.
To inspect the installer without applying it, optionally run
`ptcgl-omarchy update --check`. It compares the official MSI, not the game's
in-client patch state; checking first is not required for `update`.

The app's desktop actions include **Check installer updates**, **Update game**,
**Repair game**, and **Diagnostics**. Graphical checks, updates, and repairs show
operation-specific progress and a result dialog explaining whether the installer
was unchanged, an update is available, or the update/repair completed. These
maintenance actions require the game to be closed.

**Launcher and Proton updates are separate.** Close the game and repeat the
[launcher-only installation command](#install) to install the latest launcher.
To select an independently pinned release, use its complete verified installation
block with `--launcher-only` on the final `bash` command.

Missing releases, invalid attestations, or mismatched hashes stop installation.
The installer never falls back to building `main`.

Your selected Proton profile is retained. To explicitly select the profile
bundled with the new launcher release:

```bash
ptcgl-omarchy runtime-update
```

Previous managed runtime versions remain available for troubleshooting.
See [the update guide](docs/UPDATES.md) for repair, interrupted operations, and
runtime rollback behavior.

## Commands

### New in v0.5.0

**v0.5.0** adds live activity status, startup feedback, a compact companion,
measured download progress, recoverable interrupted installations, guided fixes,
privacy-safe support reports, and a component update overview. Existing-installation
desktop smoke testing passed, and the user confirmed the installed release
candidate's logout/browser-login and gameplay test. Clean-OS provisioning,
vertical-bar interaction, and real interrupted-install recovery remain documented
validation follow-ups; see [the validation record](docs/VALIDATION.md).
The default bootstrap and companion installer select the latest **stable** release.

The companion now requires **live status schema 1**. It prompts to upgrade when
an older launcher lacks that protocol. Its default view shows status and one
primary action, with **Maintenance**, **Folders**, and **Advanced** collapsed.
Play becomes **Focus game** while running; maintenance is disabled during game
sessions and other operations. Status refreshes only while the panel is open and
never checks the network.

Downloads show MiB, percentage and speed when the total is known, or an
indeterminate bar otherwise. Download cancellation preserves verified cache files;
verification and installer phases cannot be cancelled through the dialog. Retries
restart interrupted transfers rather than resuming partial HTTP downloads.

Use **Maintenance → Diagnostics & recovery** for confirmed, supported fixes and
**Save report** / **Copy report** (copy requires `wl-copy`). Shared reports omit
personal paths, raw diagnostic output, configuration, environment variables, and
login URLs. The local diagnostic view still includes detailed errors. To save a
report from the terminal:

```sh
ptcgl-omarchy doctor --save ptcgl-support.txt
```

Failed checks still produce exit status 1 even when saving succeeds. Use
**Maintenance → Versions & updates** or `ptcgl-omarchy updates --gui` to inspect
component versions and explicitly choose an update. An installer comparison is
labelled with its check date; it is not a claim about in-game patch freshness.

| Command | What it does |
| --- | --- |
| `ptcgl-omarchy` | Graphical first-time setup, then play |
| `play [-- <args>]` | Launch the game or focus its existing window |
| `setup [--gui]` | Prepare the selected runtime and install missing game files |
| `update [--check] [--gui]` | Check or apply the current official installer |
| `repair [--gui]` | Reapply installer-managed game files |
| `runtime-update [--gui]` | Select this launcher's bundled compatibility profile |
| `doctor [--json\|--gui]` | Check dependencies, game installation, and desktop integration |
| `doctor --save <file>` | Save a privacy-safe support report (v0.5.0+) |
| `status [--json]` | Read installation readiness, activity, and safe actions without network access (v0.5.0+) |
| `updates [--gui]` | Explain component versions and explicit update options (v0.5.0+) |
| `fix [--gui] <integration\|setup\|repair\|runtime-update>` | Run an allowlisted recovery action; GUI actions ask for confirmation (v0.5.0+) |
| `open <game\|config\|cache>` | Open the selected folder in your default file manager |
| `open --gui` | Choose a folder graphically |
| `integrate` | Refresh the app-menu entry, browser callback, and window rules |
| `uninstall` | Remove the launcher and integration; retain installed game data |

Commands in the table are subcommands of `ptcgl-omarchy`, except for the first row.

## Troubleshooting

Start with:

```bash
ptcgl-omarchy doctor
```

Failed checks show the reason and a **Fix** line with a recovery command or
instructions. Reports distinguish missing setup, incomplete/custom Proton
runtimes, missing desktop entries, incorrect browser handlers, and Hyprland
configuration errors. The app-menu **Diagnostics** action shows the same guidance.

For scripts, `ptcgl-omarchy doctor --json` includes `name`, `ok`, and `detail` for
each check, plus `fix` for failures. The command exits with status 1 if any check
fails.

- **Login opens another installation:** run `ptcgl-omarchy integrate` to select
  this launcher's browser callback handler.
- **Installer-managed game files need repair:** close the game and run
  `ptcgl-omarchy repair`.
- **A game update changes window behavior:** the Lua rules match the game's
  title and XWayland, rather than every game using `steam_app_0`. Inspect the
  game window with `hyprctl clients -j` and check `hyprctl configerrors`.
- **Maintenance reports a running session:** close the game before setup,
  update, repair, integration changes, uninstall, or runtime changes.

Setup and update output is available when run from a terminal. Login callback
arguments and callback subprocess output are not persisted by the launcher.
Unity Player logging is disabled because the current game writes login callback
URLs to its own log.

## Configuration

<details>
<summary><strong>Paths and optional launch overrides</strong></summary>

The launcher honors `XDG_DATA_HOME`, `XDG_CACHE_HOME`, `XDG_CONFIG_HOME`, and
`XDG_STATE_HOME`. Default locations:

```text
~/.local/share/ptcgl-omarchy/prefix/       Game, registry, settings, login state
~/.local/share/ptcgl-omarchy/runtime/      Managed GE-Proton versions
~/.cache/ptcgl-omarchy/                   MSI and Proton download cache
~/.config/ptcgl-omarchy/config.json       Optional user overrides
~/.local/state/ptcgl-omarchy/             Runtime profile and installation receipts
~/.config/hypr/ptcgl-omarchy.lua          Game-specific window rules
```

Example `config.json`:

```json
{
  "proton_path": "/absolute/path/to/GE-Proton",
  "extra_args": ["-screen-width", "1280", "-screen-height", "720"]
}
```

Omit `proton_path` to use the managed runtime. Custom runtimes are validated,
but maintained by you rather than by `runtime-update`.

</details>

## Development

The launcher uses Go and a pinned [Relic](https://github.com/sassoftware/relic)
library for built-in MSI verification. Build and verify it with:

```bash
make check
make test
make check-shell
make security
make build
```

The v0.4.1 security release requires Go 1.27.3 or later to include compromised-CA
rejection. `make security` checks module integrity and reachable vulnerabilities
with that selected build toolchain. `make check-shell` requires ShellCheck.
See the [security audit and patch status](docs/SECURITY-AUDIT.md).

For the shell companion, `make test-plugin` runs its dependency-free JavaScript
contract tests (Node.js 18+); `make check-plugin` validates its manifest and QML
against the installed Omarchy/Qt runtime.

With dependencies already installed, `make install-launcher` installs your local
build and refreshes desktop integration without running game setup.

Version tags trigger the release workflow, which runs the checks, generates
release-specific installation instructions, and publishes the attested installer,
manifest, and Linux x86-64 launcher. Release installation needs neither Go nor
Make nor a repository clone.

Publication also requires protected release settings and an independent rebuild
matching the launcher's exact bytes. Maintainers must configure the protections
described in [the release guide](docs/RELEASES.md#required-github-release-settings)
before the new workflow can publish.

Tests cover the release installer, generated bootstrap instructions, verified
downloads, archive extraction, update locking, prefix preservation, callback
delivery, runtime selection, and integration recovery.
Real graphics, browser authentication, in-game patching, and gameplay also need
manual validation on Omarchy.

- [Implementation plan](docs/PLAN.md)
- [Installation simplification plan](docs/INSTALLATION-PLAN.md)
- [Companion plugin plan](docs/PLUGIN-PLAN.md)
- [Game and runtime update behavior](docs/UPDATES.md)
- [Real-machine validation record](docs/VALIDATION.md)
- [Compatibility profile](internal/app/assets/compatibility.json)
- [Report an issue](https://github.com/hucKOder/omarchy-ptcgl/issues)

## Credits

Built for [Omarchy](https://omarchy.org), using
[UMU](https://github.com/Open-Wine-Components/umu-launcher) and
[GE-Proton](https://github.com/GloriousEggroll/proton-ge-custom).
MSI signature verification uses [Relic](https://github.com/sassoftware/relic)
from SAS Institute (Apache-2.0).
Run `ptcgl-omarchy licenses` to view the bundled dependency notices.

The initial compatibility profile, updater working-directory requirement, and
browser callback behavior were researched using
[PTCGL-Linux](https://github.com/mattmcbeardface/ptcgl-linux). This is an
independently implemented Go launcher.

## License

[MIT](LICENSE). The official Pokémon client and compatibility runtimes are
downloaded during setup rather than distributed in this repository.

An unofficial community project for Omarchy. Not affiliated with the Omarchy
project or The Pokémon Company.
