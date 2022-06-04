PREFIX?=/usr/local
_INSTDIR=$(PREFIX)
BINDIR?=$(_INSTDIR)/getwtxt-ng
#VERSION?=$(shell git describe --tags --abbrev=0)
#GOFLAGS?=-ldflags '-X github.com/gbmor/getwtxt-ng/common.Version=${VERSION}'

all: clean build

.PHONY: build
build:
	@printf "%s\n" "Building getwtxt-ng."
	go build $(GOFLAGS) ./cmd/getwtxt-ng
	go build ./cmd/adminPassGen
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
	go test -race ./...
	@printf "\n"
