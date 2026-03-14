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

.PHONY: all web one-api tokilake tokiame clean

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

clean:
	$(CLEAN_DIST)
	$(CLEAN_WEB)
