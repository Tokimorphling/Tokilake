NAME=one-api
TOKILAKE_NAME=tokilake
TOKIAME_NAME=tokiame
DISTDIR=dist
WEBDIR=web
VERSION=$(shell git describe --tags --always 2>/dev/null || echo "dev")
GOBUILD=go build -ldflags "-s -w -X 'one-api/common/config.Version=$(VERSION)'"

.PHONY: all web one-api tokilake tokiame clean

all: tokilake tokiame

web: $(WEBDIR)/build

$(WEBDIR)/build:
	cd $(WEBDIR) && bun install && VITE_APP_VERSION=$(VERSION) bun run build

one-api: web
	mkdir -p $(DISTDIR)
	$(GOBUILD) -o $(DISTDIR)/$(NAME) .

tokilake: web
	mkdir -p $(DISTDIR)
	$(GOBUILD) -o $(DISTDIR)/$(TOKILAKE_NAME) .

tokiame:
	mkdir -p $(DISTDIR)
	$(GOBUILD) -o $(DISTDIR)/$(TOKIAME_NAME) ./cmd/tokiame

clean:
	rm -rf $(DISTDIR) && rm -rf $(WEBDIR)/build
