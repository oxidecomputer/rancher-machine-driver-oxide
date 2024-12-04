GO = go

BINDIR := bin
BINARY := oxide-rancher-machine-driver

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
