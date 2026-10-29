# PTCGL Companion

An Omarchy Quattro bar widget for the existing Pokémon TCG Live launcher. Click
**PTCGL** (or **TCG** on a vertical bar) to open the themed companion panel.

## Requirements

- Omarchy Quattro with its Quickshell bar and `qs.Ui` / `qs.Commons` modules.
- x86-64 Linux, Lua Hyprland configuration (0.55+), XWayland/Vulkan, and 8 GB free
  for launcher/game installation.
- A matching launcher with **status schema 1**, available in **v0.5.0-rc.1** but
  not stable v0.4.1. Older launchers produce an upgrade prompt. Close the game and
  use the candidate's [pinned release instructions](https://github.com/hucKOder/omarchy-ptcgl/releases/tag/v0.5.0-rc.1)
  with `--launcher-only` on the final `bash` command, or build this checkout with
  `make install-launcher`. The panel's authenticated installer selects the latest
  **stable** release; it cannot install this candidate until a compatible stable
  launcher is published.
- Bash, coreutils (`timeout`, `realpath`, `install`), util-linux (`flock`), and
  Omarchy's configured terminal. These are system tools available on Omarchy.

The companion's version is independent of the launcher version shown in its panel.

## Install

After explicitly installing the matching candidate launcher:

```sh
omarchy plugin add https://github.com/hucKOder/omarchy-ptcgl.git --enable
```

For a local checkout, validate, copy, and enable its current files:

```sh
make install-plugin
```

Local installation needs Qt's `qmllint` and refuses to overwrite an existing
plugin directory. The local copy is independent of the checkout and is not
git-managed. To replace it after editing, remove that copy using the command
below, then run `make install-plugin` again. Omarchy backs up hand-installed
plugin directories when removing them.

If the shell retains compiled QML after replacing development files, run
`omarchy restart shell` to load the fresh source.

The widget initially lands in the right bar section. Move it with:

```sh
omarchy bar move io.github.huckoder.ptcgl --section left
```

## First launch

Adding/enabling a plugin does not install a launcher or run installation hooks.

**Release-candidate testing:** install v0.5.0-rc.1 explicitly as described above
before enabling the companion. The stable-install flow below still selects
v0.4.1, which lacks status schema 1 and will leave the companion showing its
upgrade prompt. Do not use it to upgrade or reinstall a candidate launcher.

1. Open the PTCGL panel and choose **Install launcher** when needed.
2. The configured terminal opens and runs this checkout's `bootstrap.sh` with
   `--launcher-only`. It selects the latest stable release, installs bootstrap
   tools through Omarchy, authenticates the release installer, then installs the
   verified launcher and its dependencies. Enter a sudo password in that terminal
   if Omarchy requests one. GitHub authentication is not needed.
3. The terminal retains the result until Enter is pressed. Failed or interrupted
   installs show an error and can be retried. Only one companion installation can
   run at a time.
4. Reopen the panel, or press **r** to refresh, then choose **Set up & play**.
   Existing launcher dialogs prepare Proton and the official game client, then
   launch the game.

Downloads and verification failures follow the existing
[authenticated release installation policy](RELEASES.md). System package
installation may already have completed when a later verification step fails.

## Panel controls

- Click the bar button to open/close; **Escape** or outside click dismisses.
- **Up/Down**, **j/k** select enabled actions; **Enter/Space** activates them.
- **r** refreshes version and diagnostics; **Tab/Shift+Tab** switch bar panels.
- The primary action changes between **Set up & play**, **Play**, **Focus game**,
  **Repair game**, and activity feedback. Repeated launches during startup wait
  briefly for the same session's window instead of creating another game.
- **Maintenance**, **Folders**, and **Advanced** are collapsed initially. Activate
  a section header with the mouse or Enter/Space to expand it. Refresh retains
  the selected action and section expansion rather than resetting the panel.
- **Maintenance → Versions & updates** explains installed launcher/runtime
  versions, last installer application, and the dated last installer comparison.
- **Diagnostics & recovery** offers confirmed, allowlisted fixes plus save/copy
  of a support report. Clipboard support is optional (`wl-copy`); reports omit
  raw errors, personal paths, config/environment values, and callback URLs.
- Installer checks, game updates, repairs, diagnostics, and folder shortcuts use
  the existing launcher commands and graphical error/result dialogs.
- The game must be closed for maintenance. The launcher enforces its session lock
  even if an action is selected from multiple monitors or applications.

Summon or hide the panel from a terminal or your own shortcut:

```sh
omarchy-shell shell summon io.github.huckoder.ptcgl '{}'
omarchy-shell shell hide io.github.huckoder.ptcgl
```

The panel checks the launcher protocol when opened/refreshed, then polls lightweight
local status approximately every two seconds while open. Closing it stops further
polling; an in-flight bounded query may finish. Status never performs online update
checks. It separates installation readiness from starting/running game sessions,
setup, update, repair, and unknown activity. Unavailable or contradictory status
disables maintenance rather than assuming the game is closed. Malformed launcher
configuration still produces a structured unavailable status; Diagnostics and
folder access remain available.

Activity is backed by the live session lock and its process/boot identity, not
just a session file. Abandoned markers are ignored. A surviving owned game window
also blocks maintenance if the launcher itself exited. The backend rechecks its
lock when actions execute; panel state is advisory, not authorization.

Long-running actions are detached from the shell: closing the panel, disabling
the companion, or restarting the shell does not terminate an already-started game,
installer, or maintenance action. After an operation, reopen/refresh the panel to
read its current state. During downloads the panel also reports MiB, known totals,
percentage, and speed. Cancellation belongs to the launcher's download dialog,
not to panel closure. Verification/extraction/MSI phases are non-cancellable from
that dialog. Startup feedback is independent of game lifetime: dismissing it does
not stop the game and its observer never dispatches focus.

## Updates

| Component | Update mechanism |
| --- | --- |
| Git-installed companion | `omarchy plugin update io.github.huckoder.ptcgl` |
| Locally copied companion | Remove the local plugin, then `make install-plugin` |
| Launcher | Panel's **Install / upgrade launcher…**, using authenticated stable releases |
| Official game installer | **Check installer updates** / **Update game** |
| In-client patches | The game checks them at launch |
| Proton compatibility profile | **Advanced → Select bundled runtime…**, or `ptcgl-omarchy runtime-update` |

Plugin updates do not automatically upgrade the launcher, game, or Proton.

## Removal

```sh
omarchy plugin remove io.github.huckoder.ptcgl
```

This removes the widget and plugin checkout (or backs up a hand-installed copy).
The launcher, its desktop integration, and game data have their own lifecycle.
To remove the launcher/integration as well:

```sh
ptcgl-omarchy uninstall
```

That command retains the game prefix, downloads, configuration, and runtime data.

## Development and validation

### Status protocol

`ptcgl-omarchy status --json` emits one schema-1 object and exits 0 when it can
encode a response, including an unavailable response. It writes no application
state, performs no network requests, and opens no graphical dialogs.

- `readiness`: `ready`, `setup-needed`, `repair-needed`, or `unavailable`.
- `activity`: `idle`, `starting`, `running`, `setup`, `update`, `repair`, `check`,
  `runtime-update`, `maintenance`, or `unavailable`.
- `phase`, `message`, optional `progress`: current local operation feedback;
  `progress.total = -1` means unknown. Never infer a percentage in that case.
- `capabilities`: boolean `play`, `maintain`, `diagnostics`, and `folders`.
- `launcher_version`, `runtime`, `custom_runtime`, and `installer`: installed
  component context, not an automatic online freshness check.
- `installer.freshness`: `not-checked`, `unchanged-at-check`, or
  `available-at-check`, with optional `checked_at` and `last_applied` UTC timestamps.

The companion validates the schema and capability consistency before enabling
actions. `doctor --json` retains its existing array format; failed checks may now
include an allowlisted `action` identifier in addition to human-readable `fix`.
Never evaluate the `fix` text as a command.

```sh
make check
make test
make build
make test-plugin
make check-plugin
```

`test-plugin` needs Node.js 18+ for the dependency-free JavaScript contract tests.
`check-plugin` needs an installed Omarchy runtime and Qt `qmllint`; it
validates the manifest and resolves `qs.*` imports through a temporary import map,
then treats QML warnings as failures. Set `QMLLINT` for a nonstandard lint binary.
Neither check enables the plugin.

Tests cover version compatibility, live-status protocol failures and contradictory
capabilities, readiness/activity actions, collapsed sections, measured progress,
fixed command argument arrays, paths with spaces, keyboard action navigation,
installer failure propagation, and installation locking. Launcher tests also cover
stale/PID-reuse/boot identity, repeat launch during startup, orphaned windows,
download cancellation, interrupted MSI/runtime recovery, and private report export.
Real shell interaction, gameplay, and process survival also need desktop smoke
testing. The validation below is historical, not acceptance of these new controls.

### Desktop validation, 2026-10-16

- Installed/enabled a local copy in the running Omarchy shell; checked summon,
  Escape dismissal, repeated reopen, disable/re-enable, and shell restart.
- The actual installed v0.3.0 launcher correctly produced an upgrade prompt.
- With a temporary launcher fixture, verified ready, incomplete setup, missing
  launcher, malformed JSON, and diagnostic timeout states in the real popup.
- Activated a fixture launch with Enter; the same detached process survived
  popup closure, plugin disable/re-enable, and a shell restart.
- Captured `preview.webp` from the actual panel using deterministic ready-state
  fixture data. Restored the installed companion to its normal launcher path.
- Full gameplay with v0.4.0+, real installer interactions, and vertical-bar layout
  remain manual validation before marketplace submission.

### Desktop candidate validation, 2026-10-29

The matching 0.5.0-dev launcher and companion passed live running/idle status,
keyboard expansion/dismissal, maintenance gating, both-monitor summon, game
survival through shell restart, measured/cancellable download checks, and GUI
support report saving. The game reached its signed-in home screen using retained
login state; no match or fresh browser login was attempted. The original launcher,
companion, and exact bar layout were restored afterward.

Rescan/disable/re-enable retained cached old QML/JavaScript on this host; restarting
the shell was necessary to load the candidate (and later the restored original).
Verify visible controls/version after a local update. Vertical-bar layout and a
complete gameplay acceptance remain open. See [the complete record](VALIDATION.md).

## License and marketplace

The companion uses the repository's [MIT license](../LICENSE). It reuses Omarchy
UI components from the installed system. The game, UMU, GE-Proton, and launcher
dependency notices retain their existing licenses and distribution mechanisms.

The companion is implemented in this checkout; a marketplace listing requires
publication, exact-commit validation, and maintainer approval. Proposed category:
Other; tags: games, launcher, quickshell. See [the saved implementation plan](PLUGIN-PLAN.md).
