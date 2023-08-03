PREFIX?=/usr/local
_INSTDIR=$(PREFIX)
BINDIR?=$(_INSTDIR)/getwtxt-ng
#VERSION?=$(shell git describe --tags --abbrev=0)
VERSION?=dev
GOTAGS?=-tags 'fts5'
GOFLAGS?=-ldflags '-s -w -X github.com/gbmor/getwtxt-ng/common.Version=${VERSION}'

all: clean build

.PHONY: build
build: getwtxt-ng adminPassGen bulkUserAdd

getwtxt-ng:
	@printf 'Building getwtxt-ng\n'
	go build ${GOTAGS} ${GOFLAGS} ./cmd/getwtxt-ng

adminPassGen:
	@printf 'Building adminPassGen\n'
	go build ${GOFLAGS} ./cmd/adminPassGen

bulkUserAdd:
	@printf 'Building bulkUserAdd\n'
	go build ${GOTAGS} ${GOFLAGS} ./cmd/bulkUserAdd

.PHONY: clean
clean:
	@printf 'Cleaning build.\n'
	go clean ./...
	rm -f adminPassGen
	rm -f getwtxt-ng
	rm -f bulkUserAdd

.PHONY: test
test:
	@printf 'Running tests.\n'
	go test ${GOTAGS} -race ./...

.PHONY: dev-deps
dev-deps:
	@printf 'Installing golangci-lint@latest\n'
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@printf 'Installing govulncheck@latest\n'
	go install golang.org/x/vuln/cmd/govulncheck@latest
	@printf 'Installing pre-commit hook\n'
	@if [ ! -f .git/hooks/pre-commit ]; then ln -s $(shell pwd)/pre-commit .git/hooks/pre-commit; fi
