GO = go

BINDIR := bin

# Rancher expects the the binary name to match the format
# `docker-machine-driver-*` otherwise it will error.
BINARY := docker-machine-driver-oxide

LDFLAGS := -ldflags "-w -s -extldflags '-static -Wl,--fatal-warnings'"
TAGS := "netgo osusergo no_stage static_build"

$(BINDIR)/$(BINARY): $(BINDIR) clean
	CGO_ENABLED=0 $(GO) build -tags $(TAGS) -o $@ ${LDFLAGS}

$(BINDIR):
	mkdir -p $@

.PHONY: test
test:
	$(GO) test -cover -v ./...

.PHONY: clean
clean:
	rm -f $(BINDIR)/*

.PHONY: lint
lint:
	golangci-lint run

.PHONY: lint-fix
lint-fix:
	golangci-lint run --fix
