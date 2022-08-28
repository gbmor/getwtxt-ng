PREFIX?=/usr/local
_INSTDIR=$(PREFIX)
BINDIR?=$(_INSTDIR)/getwtxt-ng
#VERSION?=$(shell git describe --tags --abbrev=0)
VERSION?=dev
GOTAGS?=-tags 'fts5'
GOFLAGS?=-ldflags '-s -w -X github.com/gbmor/getwtxt-ng/common.Version=${VERSION}'

all: clean build

.PHONY: build
build:
	@printf "%s\n" "Building getwtxt-ng."
	go build ${GOTAGS} ${GOFLAGS} ./cmd/getwtxt-ng
	go build -ldflags='-s -w' ./cmd/adminPassGen
	@printf "\n"

.PHONY: clean
clean:
	@printf "%s\n" "Cleaning build."
	go clean ./...
	rm -f adminPassGen
	rm -f getwtxt-ng
	@printf "\n"

.PHONY: test
test:
	@printf "%s\n" "Running tests."
	go test ${GOTAGS} -race ./...
	@printf "\n"
