# Security audit and incident comparison — 2026-10-22

## Scope and project status

Reviewed `main` at `e6dcd334432ad1a091446e0db9e6527bb1526b87`, including the
uncommitted companion plugin, installer scripts, Go dependencies, release
workflow, and public GitHub repository protection settings. The latest stable
release is v0.4.0, built from `45e4398fc0477be23d946a727f13bb5c41fec9f1`.
The verifier and archive-extraction code reviewed here also exists in that tag.

The user was playing during the follow-up review. Developer-tool installation,
read-only repository queries, and isolated fixtures were used. Deployment and
game-touching acceptance belong to a later closed-game session.

Functional checks passed in the initial audit: `make check`, `make test`,
`make test-plugin`, `make check-plugin`, a release-equivalent temporary build,
and `go mod verify`. Fresh v0.4.0 installer, manifest, and launcher attestations
passed the explicit repository/workflow/tag/source/runner policy, and the artifact
hashes matched the authenticated manifest. This is evidence of provenance, not
evidence that every security property is correct.

### Bulk patch status

The following changes implement the audit's first patch set and are now published
in v0.4.1. Existing v0.4.0 assets retain their original bytes.

| Finding | Source-patch status | Remaining acceptance |
| --- | --- | --- |
| SEC-01 | Complete non-root CRL checking, fail-closed missing evidence, alternate-chain handling, and permanent publisher/timestamp regressions. Installed-candidate verification and upgrade preservation passed. | Published v0.4.1 verification and authenticated upgrade passed. |
| SEC-02 | Go 1.27.3, explicit-root compromised-key regressions, and real downloader rejection passed; the scan reports zero reachable vulnerabilities. Host CA/active `gh` remediation also passed. | Patched artifacts are published; authenticated installation passed. |
| SEC-03 | Root-confined extraction, full link-graph validation, and escape/cycle/valid-link regressions passed. Fresh isolated real runtime provisioning passed. | Clean-OS dependency provisioning remains a separate acceptance task. |
| SEC-04 | Hosted metadata checks, CI-protected main ancestry, owner full-policy approval, and separate-runner reproduction passed. Solo-maintainer main/tag/environment settings are active. | Hosted jobs, exact rebuild comparison, and published assets passed verification. |
| SEC-05 | Explicit runtime/verifier environment allowlist passed synthetic and real-process credential exclusion checks. Desktop tests and user-confirmed fresh login/gameplay passed. | Same-user filesystem/network isolation and custom-overlay testing remain separate tasks. |

Release preparation now deliberately fails if the required repository settings
are absent. See [the exact settings](RELEASES.md#required-github-release-settings).
The second build is independent execution within the same GitHub trust domain,
not an external guarantee against a fully compromised repository administrator.

## Release-preparation follow-up

The saved independent review's default-branch-alias and unbound-CI-context findings
are fixed and covered by regressions. Public certificate fixtures now distinguish
the runtime's compromised-key rejection from expiry, hostname errors, and root
removal. Offline Go 1.27.2/1.27.3 comparison and real downloader checks passed.

The owner confirmed there is no second maintainer. Active GitHub controls now use
the documented solo-maintainer policy: CI-required PRs, immutable tags, and
owner-approved, tag-only publishing with admin bypass disabled. These do not
provide independent human review or protect against approval by a compromised
sole-maintainer account. The read-only policy check passes against the real settings.

Installed upgrade preservation, fresh runtime provisioning, host `curl`/`gh`
remediation, and desktop acceptance are recorded in
[the security-candidate validation record](VALIDATION.md#security-candidate-acceptance--2026-10-22).
After the reported 61/61 stall and recovery, the user confirmed a clean fresh
browser login and completed match with graphics/audio. The published v0.4.1
release at `06345c9bb9a5c2502770fca34aec3e45f3eaed9d` passed hosted and local exact
rebuild comparison, artifact authentication, and a real authenticated upgrade.
See [the published validation record](VALIDATION.md#published-v041-verification--2026-10-22).
The hosted token can check only public protection metadata; the owner's full
no-bypass audit passed before release approval, as required by the release guide.

## Current incident comparison

These are separate incidents disclosed around 20–21 October, rather than one
shared exploit. "emudrah" is interpreted as the eMudhra/emSign CA incident.
Incident details are a snapshot of the primary sources retrieved on 22 October.

| Incident | Mechanism | Applicability here |
| --- | --- | --- |
| PHP2Shell: CVE-2026-109618 and CVE-2026-109619 | OOM handling can abandon partially initialized request state; stale request-owned global state can survive into a later request, producing use-after-free and RCE. | The launcher has no PHP runtime or web-request worker pool. These CVEs do not directly apply. The transferable lesson is to bound hostile parser input, contain failure, and test cleanup/retry paths. |
| GitLab: CVE-2026-110264 | A crafted regular expression can trigger integer overflow in the native regex compiler and unauthenticated RCE under the conditions in GitLab's advisory. | The project uses GitHub Actions and has no GitLab server, Ruby/Onigmo runtime, or public endpoint compiling supplied regexes. These facts exclude the specific CVE from the launcher; they do not exclude parser or hosting-platform vulnerabilities in general. |
| eMudhra/emSign intermediates | Weak intermediate CA keys permit forged TLS certificates. Browser revocation updates and non-browser runtime/trust-store updates are different protection paths. | Directly relevant to HTTPS downloads, including the source bootstrap. Go 1.27.2 is affected by GO-2026-6809. The separate MSI intermediate-revocation defect is the same trust-failure class, but this review did not demonstrate that the specific compromised TLS intermediates can satisfy code-signing EKU constraints. |
| LiteLLM v1.107.0, 21 October | The vendor reports stolen developer GitHub CLI OAuth credentials, an owner-level protection override, a modified workflow selecting a lookalike repository, and malicious artifacts signed/published through the official pipeline. | LiteLLM is not a project dependency. Compromised maintainer credentials and an authorized malicious release are nevertheless applicable supply-chain threats. The existing provenance checks cannot make an authorized malicious workflow harmless. |

### PHP and GitLab: relevant lessons without claiming the same RCE

PHP's confirmed paths involve request lifecycle and native memory management;
GitLab's critical path involves its native regular-expression compiler. The
launcher is built with `CGO_ENABLED=0` and its production code has no inbound HTTP
listener. Its externally influenced inputs are instead MSI files, Proton
archives, certificate/CRL data, and browser callback URIs.

Downloads have compressed-byte limits; Proton extraction has an aggregate
regular-file limit; callbacks are capped at 16 KiB; MSI verification runs in a
child with a two-minute deadline. These are useful controls. A child process and
deadline do not impose a memory quota or create a filesystem/network sandbox.
No additional memory-corruption or parser-RCE exploit was established here.

### eMudhra: check the verifier actually used

The new Go release adds a compromised-public-key denylist to X.509 chain
validation. The scan reports `verifyCertificate -> x509.Certificate.Verify`,
and both the original review toolchain and the freshly authenticated v0.4.0 launcher
identify Go 1.27.2. Patched versions are Go 1.27.3 and Go 1.26.10 on that branch.
Go's HTTPS transport also relies on X.509 validation. An ordinary successful
HTTPS request or a protected browser does not establish that this path rejects
the compromised keys.

The complete installation path also uses `curl` and `gh`, whose verification
implementations and trust material need their own validation. Rebuilding the Go
launcher does not update those tools or their trust policy. Updating a compiler
does not patch an already-built binary.

The README's first bootstrap is executable source from mutable `main`, trusted
through HTTPS before release attestation runs. A successful TLS impersonation
against an unprotected client, or an authorized repository compromise, can
therefore affect this initial step. The documented independently selected
commit/release procedure reduces mutable-source exposure but still relies on
the integrity of the selected source and the provisioned verifier.

### LiteLLM: signed artifacts can still be malicious

The vendor's incident report, updated 21 October at 20:30 UTC, identifies PyPI
`litellm==1.107.0` and affected container tags/digests. It says the official source
tag was clean while published artifacts came from a lookalike repository. Its
description of credential exposure through AI response logs and access to its
internal deployment remains part of an ongoing investigation, not independently
verified attribution by this project.

Our release workflow has useful barriers: default current-repository checkout,
no external-source repository input, a build-produced artifact ID consumed by the
publisher, read-only build permissions, SHA-pinned actions, disabled checkout
credential persistence, and no execution of project code in the privileged
publish job. Installers bind attestations to an exact repository, workflow,
source ref and digest, signer digest, issuer, and hosted-runner policy.

Those barriers depend on who can change the workflow and release source. A
legitimate attestation verifies a workflow's identity and statements about the
artifact; it does not independently reproduce or inspect that artifact. The
automatically selected latest source commit is also not an independent approval
of a new workflow revision.

## Initial findings and trust boundaries

Severity below is this project's qualitative assessment. Operational control
gaps and conditional exposure are identified explicitly.

### SEC-01 — High: revoked MSI intermediate CA accepted

- Evidence: `internal/app/verify.go:94–105` builds a trusted chain, then checks
  revocation for its leaf only. This applies to publisher and timestamp chains.
- An isolated test signed an MSI with the expected publisher identity and a
  valid leaf, while the root's current signed CRL revoked the intermediate.
  `verifyMSI` accepted it. A direct revocation check rejected the intermediate,
  establishing that the fixture's revocation evidence was valid.
- Exploitation requires delivering an MSI and signing through a compromised,
  revoked intermediate that can build an otherwise valid code-signing chain.
- Fix: require acceptable revocation evidence for every non-root certificate in
  an accepted chain, with explicit policy for unavailable evidence and alternate
  chains. Cover publisher and timestamp intermediates.

### SEC-02 — Medium: reachable compromised-CA advisory in released Go runtime

- Evidence: source-aware `govulncheck@v1.9.0` reports GO-2026-6809 /
  CVE-2026-111943. Authenticated v0.4.0 binary metadata confirms Go 1.27.2.
- Fix: build with a patched supported Go toolchain and publish new artifacts;
  validate rejection of the known compromised keys through the real TLS path.
- The emergency key denylist and general chain-revocation fix solve different
  problems and are both needed. Exact eMudhra-based MSI forgery was not proven.

### SEC-03 — Low: archive symlink graph escapes staging

- Evidence: `internal/app/proton.go:187–202` checks lexical link targets and then
  follows previously created parent links during later symlink creation.
- A synthetic archive containing `GE/files/deep/root -> ../..`,
  `GE/files/escape -> deep/root/../../escape`, and
  `GE/files/escape/created -> ../target` created a symlink in an existing sibling
  `escape` directory outside staging, and extraction returned success.
- Production archives must match the embedded Proton SHA-256 first, limiting
  exposure. The demonstrated write was symlink creation, not arbitrary regular
  file replacement or a demonstrated game compromise.
- Fix: validate transitive targets and use root-confined parent operations.

### SEC-04 — High operational risk: release source has no repository protection

- Initial read-only GitHub API checks returned zero repository rulesets, `Branch not
  protected` for `main`, and zero environments. The release job also declares no
  protected environment. No `CODEOWNERS` file is present in the checkout.
- A compromised account with sufficient source/tag write privileges could change
  release code and trigger the existing tag-driven pipeline. Existing identity
  checks do not distinguish approved code from malicious code published by that
  authorized identity.
- Fix: protect `main` and release tags, require review of release-critical paths,
  minimize bypass and publisher rights, and use an appropriately protected
  publishing environment. Independently establish release source pins and
  compare a clean rebuild with the distributed artifact.
- This is a confirmed control gap and a conditional attack path, not evidence
  that this repository or any maintainer credential has been compromised.

### SEC-05 — Medium conditional exposure: runtime inherits developer secrets

- Evidence: `internal/app/process.go:77–88` copies every inherited environment
  entry unless an override replaces it. `umu()` in `internal/app/setup.go` and
  callback delivery use that helper for the game/runtime environment.
- A pure command-construction fixture, without launching a process, confirmed
  inheritance of synthetic `GH_TOKEN`, `OPENAI_API_KEY`, and
  `AWS_SECRET_ACCESS_KEY` values. This establishes exposure when those values are
  present in the launching environment, not that the current game has them or
  that anything exfiltrated them.
- Fix: establish a minimal game/runtime environment preserving required display,
  Vulkan, locale, audio, prefix, and desktop/session settings; test credential
  exclusion for play, setup, maintenance, and callback commands.
- A dedicated Wine prefix is not a security sandbox. Environment filtering alone
  does not prevent same-user access to credential files. Evaluate filesystem and
  network isolation separately, including browser callback compatibility.

## Prioritized implementation plan

Security work precedes feature work and marketplace publication. The main
[implementation plan](PLAN.md#security-priority--2026-10-22) tracks delivery.

1. **P0: patched runtime and chain revocation.** Address SEC-01 and SEC-02, including
   real TLS-path tests, publisher/timestamp chain regressions, failed-CRL behavior,
   alternate valid chains, and a newly built artifact's toolchain metadata.
2. **P1, before a new release: release-source controls.** Address SEC-04. Preserve
   fixed source selection and exact artifact-ID binding. Review workflow,
   bootstrap, installer, dependency, and release-script changes. Verify protections
   through the API and exercise wrong-repository/source/workflow rejection.
3. **P1: credential boundaries and extraction.** Address SEC-05 and SEC-03 with
   synthetic fixtures, then test compatibility on a closed-game installation.
   Include malformed MSI/CRL/tar and cancellation/error-cleanup cases.
4. **P1: ongoing detection and recovery.** Schedule read-only vulnerability checks
   between releases, track the toolchain of published binaries, and record how to
   announce and retire a malicious-but-validly-attested release. Review dependency
   updates and record independently established source/digest evidence.
5. **P2: parser budgets and tooling.** Investigate MSI worker memory limits and
   fuzz parser/lifecycle boundaries without treating memory-safe Go or a timeout
   as a complete containment guarantee. Triage ShellCheck diagnostics before
   adding a warning-level CI gate, and update developer-tool dependencies as
   upstream fixes become available.

Acceptance requires passing the existing checks plus security regressions and
source-aware vulnerability analysis on the actual supported build toolchain.
New published assets need attestation/hash verification and independent build
comparison. Deployment and full installer/runtime/browser/gameplay acceptance
follow after the user's active game session ends.

## Validation record

- Bulk-patch checks passed: Go formatting/vet/unit tests, race-enabled tests,
  ShellCheck, companion JavaScript tests, QML/manifest validation, and actionlint.
  The release-policy script also rejected the actual current repository settings
  as intended; external protections remain an activation task.
- Two isolated Git checkouts of the candidate source produced identical launcher
  bytes with Go 1.27.3, `CGO_ENABLED=0`, `-mod=readonly`, `-trimpath`, and version
  `0.4.1-security-candidate`. SHA-256:
  `cda3ff14a9be7028ae10230f5735718339feb874d7ef3efe32204377169c2701`.
  This is a local reproducibility smoke test with a dirty development VCS stamp;
  the clean tagged-source, separate-runner comparison awaits a hosted release run.
- The original Go/race/plugin/QML checks and module integrity verification passed.
- Source-aware vulnerability analysis found one reachable standard-library
  advisory and 22 additional module-level advisories without affected package
  imports/calls in the scanned Linux application. Scanning the stripped binary
  falls back to module-level precision and must not inflate reachability claims.
- Isolated temporary Go-overlay probes initially reproduced SEC-01, SEC-03, and
  SEC-05. Permanent regression tests now cover these paths and their valid cases.
- Patched Go 1.27.3 source-aware analysis reports zero reachable vulnerabilities;
  the 22 unused-module advisories remain distinct from reachable findings.
- The optional `PTCGL_TEST_PUBLIC_TRUST=1` smoke test exercised the real downloader
  TLS path against both eMudhra test hosts. Both failures were certificate
  verification errors with known compromised keys present in the received chain,
  not merely DNS/network failures. Offline tests also reject untrusted TLS before
  content verification or cache replacement. No private CA key is downloaded.
- The temporary candidate's private MSI worker successfully authenticated the
  existing official cached MSI with complete-chain CRL checks. This reads the MSI
  and network revocation evidence; it does not invoke Windows Installer.
- Fresh published installer, manifest, and launcher attestations/hashes passed.
  The downloaded launcher reports the expected v0.4.0 source and Go 1.27.2.
- OpenCode 1.19.0 now resolves explicit Go and Bash LSP commands in the user's
  global config. `gopls` v0.23.0 was already installed. Bash LSP v5.8.1 was
  installed in a user-local locked npm dependency tree with install scripts
  disabled; ShellCheck v0.11.0 was installed user-locally after checking its
  release archive against GitHub's published SHA-256.
- Fresh OpenCode debug clients returned clean Go diagnostics for `verify.go` and
  a genuine ShellCheck diagnostic for `scripts/plugin-install.sh`. The existing
  interactive OpenCode session needs a restart to load the new configuration.
- The initial ShellCheck run reported SC2174 in `plugin-install.sh`
  (`mkdir -p -m` parent-mode semantics), and SC2034 in `plugin-local.sh` (unused
  retry variable). The bulk patch resolves both and `make check-shell` passes.
- The developer-only npm tree reports four high-severity dependency entries for
  one upstream `braces` stack-exhaustion advisory, CVE-2026-93687. The advisory
  lists no patched release. Bash LSP's glob inputs are supplied by its config;
  the configured inputs are fixed simple patterns, background discovery is
  limited to 100 files, and build/Git/dependency directories are excluded. This
  reduces exposure but is not a dependency fix. Track the upstream fix; this npm
  tree is outside the repository and is not bundled in launcher releases.

## Primary sources

1. [PHP security advisory, 21 October](https://www.php.net/index.php#2026-10-21-5)
   and the detailed [OOM](https://github.com/php/php-src/security/advisories/GHSA-w8hw-w9fx-j79w)
   / [stale-state](https://github.com/php/php-src/security/advisories/GHSA-6wmf-4rv5-p3rp)
   advisories.
2. [NCSC-NL PHP2Shell operational information](https://github.com/NCSC-NL/php2shell).
3. [GitLab critical patch release, 21 October](https://docs.gitlab.com/releases/patches/patch-release-gitlab-19-5-1-released/).
4. [eMudhra incident statement, updated 21 October](https://emudhra.com/en/post/certificate-security-incident).
5. [Cloudflare's eMudhra response and verifier guidance](https://blog.cloudflare.com/emudhra-certificate-compromise/).
6. [Go advisory GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809),
   [Go issue 82345](https://go.dev/issue/82345), and the
   [Go 1.27.3 key-denylist implementation](https://github.com/golang/go/blob/go1.27.3/src/crypto/x509/verify.go).
7. [LiteLLM's 21 October v1.107.0 incident report](https://docs.litellm.ai/blog/security-incident-oct-21-2026).
8. [OpenCode LSP configuration](https://opencode.ai/docs/lsp/) and
   [published schema](https://opencode.ai/config.json).
9. [Developer-tool braces advisory](https://github.com/advisories/GHSA-vfj7-8cjw-p6xm).
