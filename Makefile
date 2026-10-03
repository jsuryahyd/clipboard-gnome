# Makefile for Clipboard-Gnome

VERSION ?= 1.0.1
BUILD_DATE = $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS = -X 'main.Version=$(VERSION)' -X 'main.BuildDate=$(BUILD_DATE)'

EXTENSION_UUID = clipboard-gnome@surya.dev
EXTENSION_DIR = $(HOME)/.local/share/gnome-shell/extensions/$(EXTENSION_UUID)
BUILD_DIR = build
GO_BIN = $(shell which go 2>/dev/null || echo "/usr/local/go/bin/go")
WAILS_BIN = $(shell which wails 2>/dev/null || echo "$(HOME)/go/bin/wails")

.PHONY: all test build extension install dev clean

all: test build extension

test:
	@echo "==> Running Go unit tests..."
	@export PATH=/home/linuxbrew/.linuxbrew/bin:/usr/local/go/bin:$(PATH); go test -v . ./internal/...

build:
	@echo "==> Building Wails binary (version $(VERSION))..."
	@export PKG_CONFIG_PATH=/home/linuxbrew/.linuxbrew/lib/pkgconfig:$$PKG_CONFIG_PATH; \
	export PATH=/home/linuxbrew/.linuxbrew/bin:/usr/local/go/bin:$(HOME)/go/bin:$(PATH); \
	if command -v wails >/dev/null 2>&1; then \
		wails build -clean -ldflags "$(LDFLAGS)"; \
	else \
		go build -ldflags "$(LDFLAGS)" -o build/bin/clipboard-gnome .; \
	fi

extension:
	@echo "==> Compiling schemas and packaging GNOME Shell Extension..."
	@mkdir -p $(BUILD_DIR)
	@glib-compile-schemas extension/schemas
	@cd extension && zip -r ../$(BUILD_DIR)/clipboard-gnome-extension.zip .

install: extension
	@echo "==> Installing GNOME Shell Extension locally..."
	@rm -rf $(EXTENSION_DIR)
	@mkdir -p $(EXTENSION_DIR)
	@cp -r extension/* $(EXTENSION_DIR)/
	@glib-compile-schemas $(EXTENSION_DIR)/schemas
	@echo "GNOME Extension installed to $(EXTENSION_DIR)"
	@echo "To enable, run: gnome-extensions enable $(EXTENSION_UUID)"

clean:
	@echo "==> Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@go clean
