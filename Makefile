# rawhttp CLI — build & install (no root required, macOS + Linux)
#
# The CLI lives in its own Go module (cmd/rawhttp) so the library stays
# minimal-dependency. Because of that, `go build ./cmd/rawhttp` from the repo
# root does NOT work — use these targets instead.
#
#   make build                 # compile ./rawhttp
#   make install               # install to ~/.local/bin (override with BINDIR=...)
#   make install BINDIR=~/bin  # install elsewhere
#   make uninstall
#   make clean

BINARY  := rawhttp
CLI_DIR := cmd/rawhttp
GO      ?= go

# Per-user install dir — no root required. Override on the command line.
BINDIR  ?= $(HOME)/.local/bin

.PHONY: all build install uninstall clean tidy vet test help

all: build

## build: compile the CLI binary into ./$(BINARY)
build:
	cd $(CLI_DIR) && $(GO) build -o "$(CURDIR)/$(BINARY)" .
	@echo "Built $(CURDIR)/$(BINARY)"

## install: build, then copy the binary into BINDIR (default ~/.local/bin)
install: build
	@mkdir -p "$(BINDIR)"
	@cp "$(CURDIR)/$(BINARY)" "$(BINDIR)/$(BINARY)"
	@chmod 0755 "$(BINDIR)/$(BINARY)"
	@echo "Installed -> $(BINDIR)/$(BINARY)"
	@case ":$$PATH:" in \
	  *":$(BINDIR):"*) echo "($(BINDIR) is already on your PATH)";; \
	  *) printf '\nNOTE: %s is not on your PATH. Add this to your shell rc:\n  export PATH="%s:$$PATH"\n' "$(BINDIR)" "$(BINDIR)";; \
	esac

## uninstall: remove the installed binary
uninstall:
	@rm -f "$(BINDIR)/$(BINARY)"
	@echo "Removed $(BINDIR)/$(BINARY)"

## clean: remove the locally built binary
clean:
	@rm -f "$(CURDIR)/$(BINARY)"
	@echo "Cleaned"

## tidy: go mod tidy for both modules (library + CLI)
tidy:
	$(GO) mod tidy
	cd $(CLI_DIR) && $(GO) mod tidy

## vet: vet both modules
vet:
	$(GO) vet ./pkg/...
	cd $(CLI_DIR) && $(GO) vet .

## test: run library tests
test:
	$(GO) test ./pkg/...

## help: list available targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed -e 's/^## //'
