PREFIX?=/usr/local
_INSTDIR=$(PREFIX)
BINDIR?=$(_INSTDIR)/getwtxt-ng
#VERSION?=$(shell git describe --tags --abbrev=0)
VERSION?=dev
GOTAGS?=-tags 'fts5'
GOFLAGS?=-ldflags '-s -w -X github.com/gbmor/getwtxt-ng/common.Version=${VERSION}'
GOFLAGSLITE?=-ldflags '-s -w'

all: clean build

.PHONY: build
build: getwtxt-ng adminPassGen bulkUserAdd

getwtxt-ng:
	@printf "Building getwtxt-ng\n"
	go build ${GOTAGS} ${GOFLAGS} ./cmd/getwtxt-ng
	@printf "\n"

adminPassGen:
	@printf "Building adminPassGen\n"
	go build ${GOFLAGSLITE} ./cmd/adminPassGen
	@printf "\n"

bulkUserAdd:
	@printf "Building bulkUserAdd\n"
	go build ${GOTAGS} ${GOFLAGSLITE} ./cmd/bulkUserAdd
	@printf "\n"

.PHONY: clean
clean:
	@printf "%s\n" "Cleaning build."
	go clean ./...
	rm -f adminPassGen
	rm -f getwtxt-ng
	rm -f bulkUserAdd
	@printf "\n"

.PHONY: test
test:
	@printf "%s\n" "Running tests."
	go test ${GOTAGS} -race ./...
	@printf "\n"
