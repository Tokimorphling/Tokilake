ifeq ($(OS),Windows_NT)
SHELL := $(ComSpec)
.SHELLFLAGS := /C
EXE_SUFFIX := .exe
VERSION := $(shell git describe --tags --always 2>NUL || echo dev)
MKDIR_DIST = if not exist "$(DISTDIR)" mkdir "$(DISTDIR)"
CLEAN_DIST = if exist "$(DISTDIR)" rmdir /S /Q "$(DISTDIR)"
CLEAN_WEB = if exist "$(WEBDIR)\build" rmdir /S /Q "$(WEBDIR)\build"
BUILD_WEB = cd $(WEBDIR) && corepack yarn install --frozen-lockfile && set "VITE_APP_VERSION=$(VERSION)" && corepack yarn run build
else
SHELL := /bin/sh
.SHELLFLAGS := -c
EXE_SUFFIX :=
VERSION := $(shell git describe --tags --always 2>/dev/null || echo dev)
MKDIR_DIST = mkdir -p "$(DISTDIR)"
CLEAN_DIST = rm -rf "$(DISTDIR)"
CLEAN_WEB = rm -rf "$(WEBDIR)/build"
BUILD_WEB = cd $(WEBDIR) && corepack yarn install --frozen-lockfile && VITE_APP_VERSION=$(VERSION) corepack yarn run build
endif

NAME=one-api
TOKILAKE_NAME=tokilake
TOKIAME_NAME=tokiame
DISTDIR=dist
WEBDIR=web
GOBUILD=go build -ldflags "-s -w -X 'one-api/common/config.Version=$(VERSION)'"

# Cross-compilation settings
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64
BUILD_TARGETS := $(addprefix build-,$(subst /,_,$(PLATFORMS)))
TOKILAKE_TARGETS := $(addprefix tokilake-,$(subst /,_,$(PLATFORMS)))
TOKIAME_TARGETS := $(addprefix tokiame-,$(subst /,_,$(PLATFORMS)))

.PHONY: all web one-api tokilake tokiame clean release $(BUILD_TARGETS) $(TOKILAKE_TARGETS) $(TOKIAME_TARGETS)

all: tokilake tokiame

web: $(WEBDIR)/build

$(WEBDIR)/build:
	$(BUILD_WEB)

one-api: web
	$(MKDIR_DIST)
	$(GOBUILD) -o "$(DISTDIR)/$(NAME)$(EXE_SUFFIX)" .

tokilake: web
	$(MKDIR_DIST)
	$(GOBUILD) -o "$(DISTDIR)/$(TOKILAKE_NAME)$(EXE_SUFFIX)" .

tokiame:
	$(MKDIR_DIST)
	$(GOBUILD) -o "$(DISTDIR)/$(TOKIAME_NAME)$(EXE_SUFFIX)" ./cmd/tokiame

# Cross-compilation targets
all-binaries: $(TOKILAKE_TARGETS) $(TOKIAME_TARGETS)

$(TOKILAKE_TARGETS): web
	$(MKDIR_DIST)
	$(eval PLATFORM := $(subst _,/,$(subst tokilake-,,$@)))
	$(eval GOOS := $(word 1,$(subst /, ,$(PLATFORM))))
	$(eval GOARCH := $(word 2,$(subst /, ,$(PLATFORM))))
	$(eval BIN_NAME := $(TOKILAKE_NAME)-$(GOOS)-$(GOARCH))
	@if [ "$(GOOS)" = "windows" ]; then \
		GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) -o "$(DISTDIR)/$(BIN_NAME).exe" . ; \
	else \
		GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) -o "$(DISTDIR)/$(BIN_NAME)" . ; \
	fi

$(TOKIAME_TARGETS):
	$(MKDIR_DIST)
	$(eval PLATFORM := $(subst _,/,$(subst tokiame-,,$@)))
	$(eval GOOS := $(word 1,$(subst /, ,$(PLATFORM))))
	$(eval GOARCH := $(word 2,$(subst /, ,$(PLATFORM))))
	$(eval BIN_NAME := $(TOKIAME_NAME)-$(GOOS)-$(GOARCH))
	@if [ "$(GOOS)" = "windows" ]; then \
		GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) -o "$(DISTDIR)/$(BIN_NAME).exe" ./cmd/tokiame ; \
	else \
		GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) -o "$(DISTDIR)/$(BIN_NAME)" ./cmd/tokiame ; \
	fi

# Advanced release packaging (for NPM/CI)
release-archives: all-binaries
	$(eval PKG_VERSION := $(shell grep '"version":' packaging/npm/tokiame/package.json | cut -d'"' -f4))
	@for p in $(PLATFORMS); do \
		os=$$(echo $$p | cut -d'/' -f1); \
		arch=$$(echo $$p | cut -d'/' -f2); \
		if [ "$$os" = "windows" ]; then \
			cd $(DISTDIR) && zip -j "tokiame_$${PKG_VERSION}_$${os}_$${arch}.zip" "tokiame-$${os}-$${arch}.exe" && cd ..; \
		else \
			cp "$(DISTDIR)/tokiame-$$os-$$arch" "$(DISTDIR)/tokiame"; \
			cd $(DISTDIR) && tar -czf "tokiame_$${PKG_VERSION}_$${os}_$${arch}.tar.gz" "tokiame" && rm -f "tokiame" && cd ..; \
		fi; \
	done
	@# Generate checksums
	@cd $(DISTDIR) && \
		sha256sum tokiame_$(PKG_VERSION)_linux_*.tar.gz > SHA256SUMS-linux.txt && \
		sha256sum tokiame_$(PKG_VERSION)_darwin_*.tar.gz > SHA256SUMS-darwin.txt && \
		sha256sum tokiame_$(PKG_VERSION)_windows_*.zip > SHA256SUMS-windows.txt

clean:
	$(CLEAN_DIST)
	$(CLEAN_WEB)
