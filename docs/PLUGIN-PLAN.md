# PTCGL Companion implementation plan

## Goal

Provide an Omarchy Quattro bar widget with a themed panel for installing,
launching, and maintaining Pokémon TCG Live through the existing Go launcher.
The companion lives in this repository, with a root manifest suitable for the
Omarchy plugin marketplace.

## Plugin contract and layout

- Plugin ID: `io.github.huckoder.ptcgl` (check marketplace availability before submission).
- Display name: PTCGL Companion.
- Kind: `bar-widget`, one instance, with `plugin/BarWidget.qml` as its entry point.
- `plugin/Panel.qml` is loaded by the widget rather than declared as another kind.
- Companion version is independent of the installed launcher version.
- Use Omarchy's shared shell, UI components, theme, keyboard handling, and panel lifecycle.

Planned files:

```text
manifest.json
plugin/BarWidget.qml
plugin/Panel.qml
plugin/Model.js
scripts/plugin-install.sh
docs/PLUGIN.md
preview.webp
```

Update the README, Makefile, and CI checks as needed.

## User experience

An unobtrusive PTCGL bar button opens a panel with launcher version, readiness,
a primary Install launcher / Set up & play / Play action, maintenance actions,
folder shortcuts, and explicit launcher installation/upgrades.

Refresh asynchronously when the panel opens and when requested. Distinguish
missing launcher, incomplete setup, successful checks, diagnostic failures, and
unavailable or malformed diagnostics. A diagnostics exit code of 1 may still
contain a valid JSON report. Do not treat a successful diagnostic query as proof
that the game is currently running or that a newer installer is available.

Resolve the installed launcher at `$HOME/.local/bin/ptcgl-omarchy`, matching the
existing installer, and preserve XDG environment variables.

## Existing launcher actions

| Action | Command |
| --- | --- |
| Play / set up & play | `ptcgl-omarchy app` |
| Check installer updates | `ptcgl-omarchy update --check --gui` |
| Update game | `ptcgl-omarchy update --gui` |
| Repair game | `ptcgl-omarchy repair --gui` |
| Diagnostics | `ptcgl-omarchy doctor --gui` |
| Open folder | `ptcgl-omarchy open game`, `config`, or `cache` |

Game, setup, and maintenance actions must outlive the panel, plugin reload, or
shell restart. Only short read-only status queries belong to QML-managed
processes. Existing launcher locking remains authoritative for concurrency.

## Installation and upgrades

1. Adding/enabling the plugin registers the widget; it does not install a launcher.
2. When missing or incompatible, offer an explicit launcher installation/upgrade.
3. Open the configured terminal and invoke a local helper, which runs the checked-out
   `bootstrap.sh --launcher-only` and preserves a useful success/failure result.
4. Bootstrap performs package installation and verifies release artifacts using
   the existing attestation policy. Do not introduce another download-to-shell path.
5. Refresh the panel after installation; Set up & play invokes existing graphical
   setup and launch.
6. Require launcher v0.4.0 or newer and handle incompatible versions explicitly.

Updates remain separate: Omarchy updates the companion checkout; the explicit
bootstrap action upgrades the launcher; launcher commands update the official
game installer; the game applies its own in-client patches.

Removing the companion removes the widget/checkout only. Launcher removal remains
`ptcgl-omarchy uninstall`, which retains game data. Document both lifecycles.

## Implementation milestones

1. **Shell UI:** manifest, bar button, themed popup, keyboard navigation, Escape,
   panel switching, and shell summon/hide support.
2. **Launcher integration:** version detection, diagnostics-based status, Play,
   graphical maintenance, and folder actions.
3. **Installation:** terminal helper, bootstrap reuse, missing/older launcher
   handling, and recoverable error messages.
4. **Validation/listing:** automated checks, real-desktop validation, documentation,
   preview image, and marketplace submission preparation.

The first milestone worth trying is an installed-launcher panel that opens the
game and exposes maintenance actions.

## Verification

Run existing checks:

```sh
make check
make test
make build
```

Validate against the installed Omarchy runtime:

```sh
omarchy plugin validate .
qmllint -I "$OMARCHY_PATH/shell" plugin/BarWidget.qml plugin/Panel.qml
```

Add meaningful coverage for diagnostic parsing, installation-helper argument
handling/failure behavior, and compatibility checks. Exercise missing launcher,
incomplete setup, malformed configuration, repeated Play, maintenance while busy,
disable/re-enable, hot reload, shell restart, detached operation survival, paths
with spaces, custom XDG directories, and horizontal/vertical bars.

## Marketplace preparation

Provide root README installation/removal instructions, license and dependencies,
and an optimized panel preview. Suggested category: Other; tags: games, launcher,
quickshell. Check ID availability and run marketplace compatibility/baseline
checks early because this repository includes installation/package-management
functionality. Submit only a tested published commit with owner approval.

Marketplace publication and release publication are separate follow-up actions.
Record completed implementation and validation in this plan and `docs/PLUGIN.md`.

## Implementation record, 2026-10-16

Implemented the root manifest, bar widget/panel, readiness and compatibility
model, detached launcher actions, installation helper with concurrency locking,
local-copy installer, QML/manifest checks, JavaScript contract tests, installer
tests, documentation, and preview image. CI and release builds run the companion
contract tests. `make install-plugin` installs the unpublished local source.

Passed `make check`, `make test`, `make build`, `make test-plugin`, and
`make check-plugin`. Live shell checks covered the real installed v0.3.0 upgrade
prompt, fixture-backed readiness/failure/timeout states, keyboard activation and
Escape, reopen, disable/re-enable, and detached process survival across a shell
restart. The development shell retained compiled nested QML on some source edits;
a shell restart loaded the fresh files. See `docs/PLUGIN.md` for the test record.

Remaining before marketplace submission: full v0.4.0+ gameplay/installation
validation, vertical-bar checks, marketplace ID availability/baseline review,
publishing the companion source, and owner-approved submission of a tested commit.

## Security prerequisite — 2026-10-22

Complete the release-blocking work in the
[main security plan](PLAN.md#security-priority--2026-10-22) before marketplace
publication. The [incident comparison](SECURITY-AUDIT.md) records certificate
trust, release-source, runtime-credential, and extraction findings. Companion
installation reuses the source bootstrap, so its first trusted code and
`curl`/`gh` trust paths are part of acceptance. Run installed-plugin and launcher
acceptance after the current gameplay session ends.

The bulk patch prepares the launcher fixes and CI/release gates in source. The
companion installer now uses a private umask for its state/lock files, and its
shell warnings are resolved. Publication still requires GitHub protection
activation and closed-game validation of the patched installed launcher.
