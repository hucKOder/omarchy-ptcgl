# Contributing

Use the Go version specified in `go.mod`. The Makefile pins the Go formatting and
lint tools; their first run downloads them through the Go module proxy.

Before submitting a change, run:

```sh
make fmt       # apply goimports and gofumpt
make fmt-check # verify formatting without changing files
make lint      # static analysis
make check     # Go tests, vet, generated notices and shell syntax
make test      # Go tests with the race detector
make check-shell
make test-plugin
make security
make build
```

Keep changes small and focused. Use short, responsibility-oriented package names
under `internal`; the command should remain a thin entry point. Prefer concrete
types and small interfaces defined by their consumers. Pass `context.Context` to
operations that can be cancelled. Wrap errors with useful context using `%w`,
but never include callback URLs, credentials or subprocess arguments in errors
or diagnostics. Handle errors where they affect correctness; document intentional
best-effort cleanup and narrowly justify any lint suppression.

Tests should cover behavior and failure paths, use isolated temporary directories,
and avoid real network or desktop dependencies by default. Preserve the CLI and
JSON contracts, existing state files, installation security checks, and privacy
guarantees. Changes affecting the desktop, launch, callback or installer flows
also need appropriate manual validation as described in `docs/VALIDATION.md`.

## Go package layout

- `cmd/ptcgl-omarchy` starts the CLI and installs signal handling.
- `internal/app` dispatches commands and coordinates the installer, game
  lifetime, desktop, diagnostics and persisted operation state.
- `internal/cli` owns help text, flag parsing and option validation. It never
  passes callback URLs to the flag parser.
- `internal/config` resolves user paths, validates configuration and writes files
  atomically. `internal/download` handles bounded verified transfers.
  `internal/process` runs subprocesses and filters their inherited environment.
- `internal/desktop` renders embedded desktop entries and managed configuration
  fragments; system changes and rollback remain in `internal/app`.
- `internal/verify` authenticates the MSI, signing chains, timestamps and
  revocation evidence. The private verification worker calls it using system roots.
- `internal/installer` tests the shell-based release installer.

The application currently retains compatibility aliases for extracted types.
When considering further extraction, preserve the verification worker, embed
paths, lock ownership, persisted JSON schemas, and CLI privacy guarantees. Move
desktop behavior only with its tests and assets, and avoid introducing package
cycles or broad APIs merely to move files.
