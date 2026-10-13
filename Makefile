GO ?= go
VERSION ?= 0.1.0

.PHONY: build test check install install-launcher uninstall clean
build:
	mkdir -p build
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags '-s -w -X ptcgl-omarchy/internal/app.Version=$(VERSION)' -o build/ptcgl-omarchy ./cmd/ptcgl-omarchy

test:
	$(GO) test -race ./...

check:
	bash -n install.sh
	$(GO) vet ./...
	$(GO) test ./...
	test -z "$$(gofmt -l cmd internal)"

install:
	bash ./install.sh

install-launcher: build
	./build/ptcgl-omarchy install

uninstall:
	"$$HOME/.local/bin/ptcgl-omarchy" uninstall

clean:
	rm -rf build
