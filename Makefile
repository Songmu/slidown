PKG = github.com/Songmu/slidown
COMMIT = $(shell git rev-parse --short HEAD)

BUILD_LDFLAGS = "-s -w -X $(PKG)/version.Revision=$(COMMIT)"

default: test

ci: depsdev test

test:
	go test ./... -coverprofile=coverage.out -covermode=count -count=1

build:
	go build -ldflags=$(BUILD_LDFLAGS) -trimpath -o slidown cmd/slidown/main.go

install:
	go install -ldflags=$(BUILD_LDFLAGS) -trimpath ./cmd/slidown

lint:
	golangci-lint run ./...

fuzz:
	go test -fuzz=FuzzParse -fuzztime=1m ./md/.

depsdev:
	go install github.com/Songmu/gocredits/cmd/gocredits@latest

prerelease_for_tagpr: depsdev
	go mod download
	gocredits -w .
	git add CHANGELOG.md CREDITS go.mod go.sum

.PHONY: default ci test build install lint fuzz depsdev prerelease_for_tagpr
