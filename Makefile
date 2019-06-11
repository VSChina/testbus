PACKAGE  		= github.com/devigned/testbus
DATE    		?= $(shell date +%FT%T%z)
VERSION 		?= $(shell git rev-list -1 HEAD)
SHORT_VERSION 	?= $(shell git rev-parse --short HEAD)

all: build

build:
	go build -ldflags "-X $(PACKAGE)/cmd.GitCommit=$(VERSION)" -o ./bin/testbus

build-debug:
	go build -o ./bin/testbus -tags debug

gox:
	gox -osarch="darwin/amd64 windows/amd64 linux/amd64" -ldflags "-X $(PACKAGE)/cmd.GitCommit=$(VERSION)" -output "./bin/$(SHORT_VERSION)/{{.Dir}}_{{.OS}}_{{.Arch}}"
