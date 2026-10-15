# Provisioning osslsigncode

`osslsigncode` verifies the official Pokémon MSI's signature, certificate chain,
and publisher before the launcher runs it. It is a trusted dependency: a modified
verifier could undermine those checks.

The project installer requires it on `PATH` before installing dependencies,
downloading a launcher, or building a local checkout. This applies to normal and
`--launcher-only` installation. It does not fetch, build, or install AUR packages.

## Check an existing installation

```bash
command -v osslsigncode
pacman -Qi osslsigncode
```

If you already provisioned a reviewed copy, rerun the PTCGL installer normally.
The preflight check establishes availability; it does not authenticate the binary
or approve every package with that name. Check that the executable resolved by
your `PATH` is the one you intended to install.

## Review before building

The expected packaging repository is
<https://aur.archlinux.org/osslsigncode.git>. The upstream project is
<https://github.com/mtrojnar/osslsigncode>.

1. Retrieve the packaging as data and record the exact Git commit you are reviewing.
   Read the complete `PKGBUILD`, any installation scripts, patches, and other
   supplied files. For an update, compare all changes against the previously
   reviewed revision, including changes in dependencies and source locations.
2. Check that source URLs and versions refer to the intended upstream release.
   Inspect source integrity checks and upstream signatures when available. A hash
   supplied by the same unreviewed packaging is not independent proof of origin.
3. Review AUR build/runtime dependencies too. Treat new maintainers, new remote
   scripts, obfuscated commands, or changes outside the build/package directories
   as reasons to investigate before proceeding.

Read the packaging **before** running `makepkg`, including source-verification
operations: a PKGBUILD is shell code and its top-level statements can execute when
it is loaded. A legitimate package name or upstream URL does not make its build
or installation scripts trustworthy.

## Build and install the reviewed revision

Build the exact reviewed revision in an isolated, disposable Arch environment,
such as a clean build chroot or VM. Keep source verification enabled. The build
environment should not expose your SSH keys, cloud tokens, writable project
directories, or host Docker socket.

Inspect the resulting package's file list and installation script before installing
it on your workstation. Build isolation protects the build environment; it does
not make a malicious package safe to install as root. Install only the selected,
reviewed artifact through an interactive pacman transaction, keeping dependency
and conflict checks enabled.

Keep a short record of:

- Package name and version.
- Reviewed packaging commit and relevant dependency revisions.
- Source checksums/signatures checked during review.
- SHA-256 of the resulting artifact being installed.
- Review date and noteworthy changes.

After provisioning the verifier, run the PTCGL installer again. Repeat the review
for later packaging changes rather than treating one reviewed revision as approval
of all future releases.

## Omarchy maintenance

Continue using Omarchy's official update workflow. This project change only
removes PTCGL's automatic AUR dependency installation. It does not configure
package holds or control whether the official updater later updates installed AUR
packages.

## References

- [Arch User Repository](https://wiki.archlinux.org/title/Arch_User_Repository)
- [Building in a clean chroot](https://wiki.archlinux.org/title/DeveloperWiki:Building_in_a_clean_chroot)
