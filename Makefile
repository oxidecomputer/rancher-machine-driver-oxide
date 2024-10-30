BINARY := docker-machine-driver-oxide

BUILD=`date +%FT%T%z`

LDFLAGS=-ldflags "-w -s -extldflags '-static'"

sha: build check
	shasum -a 256 ./build/docker-machine-driver-oxide
.PHONY: sha

build: docker-machine-driver-oxide
.PHONY: build

deps:
	go install honnef.co/go/tools/cmd/staticcheck@latest
.PHONY: deps

test: $(SOURCES) clean-test
	go test -v \
	  -coverpkg=./... \
	  -coverprofile=build/c.out \
	  ./...
.PHONY: test

clean-test:
	@rm -rf testdata/working/*
.PHONY: clean-test

clean: clean-test
	@rm -rf build
.PHONY: clean

local: 
	CGO_ENABLED=0 \
	  go build -o build/$(BINARY) \
	  ${LDFLAGS} \
	  cmd/docker-machine-driver-oxide/main.go
.PHONY: local

check: ## Static Check Golang files
	@staticcheck ./...
.PHONY: check

vet: ## go vet files
	@go vet ./...
.PHONY: vet

coverage: test cover
.PHONY: coverage

cover:
	go tool cover \
	  -html=build/c.out \
	  -o build/coverage/index.html
.PHONY: cover-all

$(BINARY): $(SOURCES)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	  go build -o build/$(BINARY) \
	  ${LDFLAGS} \
	  cmd/docker-machine-driver-oxide/main.go
