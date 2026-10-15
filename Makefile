GO ?= go
VERSION ?= 0.4.0

.PHONY: build test check licenses install install-launcher uninstall clean
build:
	mkdir -p build
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags '-s -w -X ptcgl-omarchy/internal/app.Version=$(VERSION)' -o build/ptcgl-omarchy ./cmd/ptcgl-omarchy

test:
	$(GO) test -race ./...

check:
	bash -n bootstrap.sh install.sh scripts/release-assets.sh
	$(GO) run scripts/license-notices.go --check
	$(GO) vet ./...
	$(GO) test ./...
	test -z "$$(gofmt -l cmd internal)"

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
