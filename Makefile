GO ?= go
VERSION ?= 0.5.0

.PHONY: build test check check-shell security test-plugin check-plugin install-plugin licenses install install-launcher uninstall clean
build:
	mkdir -p build
	CGO_ENABLED=0 $(GO) build -mod=readonly -trimpath -ldflags '-s -w -X ptcgl-omarchy/internal/app.Version=$(VERSION)' -o build/ptcgl-omarchy ./cmd/ptcgl-omarchy

test:
	$(GO) test -race ./...

check:
	bash -n bootstrap.sh install.sh scripts/release-assets.sh scripts/plugin-install.sh scripts/plugin-local.sh scripts/check-plugin.sh scripts/check-release-policy.sh
	$(GO) run scripts/license-notices.go --check
	$(GO) vet ./...
	$(GO) test ./...
	test -z "$$(gofmt -l cmd internal)"

check-shell:
	shellcheck -S warning bootstrap.sh install.sh scripts/*.sh

security:
	$(GO) mod verify
	$(GO) run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...

test-plugin:
	node --test scripts/tests/plugin-model.test.cjs

check-plugin:
	bash scripts/check-plugin.sh

install-plugin: check-plugin
	bash scripts/plugin-local.sh

licenses:
	$(GO) run scripts/license-notices.go

install:
	bash ./install.sh --local

install-launcher: build
	./build/ptcgl-omarchy install

uninstall:
	"$$HOME/.local/bin/ptcgl-omarchy" uninstall

clean:
	rm -rf build
