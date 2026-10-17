.PHONY: dev playground test test-playground test-desktop test-notifications test-media-windows build build-windows check

ifneq ($(filter dev test test-desktop test-notifications test-media-windows build build-windows check,$(or $(MAKECMDGOALS),dev)),)
MYGO_MODFILE := $(shell go run ./tools/mygo-patch)
ifeq ($(strip $(MYGO_MODFILE)),)
$(error Failed to prepare MyGo Windows patches)
endif
export GOFLAGS := $(GOFLAGS) '-modfile=$(MYGO_MODFILE)'
endif

ifneq ($(filter test test-desktop test-notifications test-media-windows check,$(MAKECMDGOALS)),)
TEST_OVERLAY := $(shell go run ./tools/test-overlay)
ifeq ($(strip $(TEST_OVERLAY)),)
$(error Failed to prepare Go test overlay)
endif
export GOFLAGS := $(GOFLAGS) '-overlay=$(TEST_OVERLAY)'
endif

dev:
	go tool mygo dev

playground:
	node playground/server.mjs

test-playground:
	npm --prefix playground run check
	npm --prefix playground test

test:
	go test -race $(TEST_ARGS) ./...

test-desktop:
	go test -race -p 1 -tags desktop -count=1 -timeout=120s $(TEST_ARGS) ./...

test-notifications:
	node tests/playground/macos/notifications.mjs

test-media-windows:
	node tests/playground/windows/media.mjs

build:
	go tool mygo build -skip-dmg

build-windows:
	go tool mygo build -platform windows/amd64,windows/arm64 -o build -skip-dmg

check:
	go vet ./...
	go tool mygo vet
	git diff --check
