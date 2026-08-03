BINARY := pwstore
VERSION ?= 1.0.0
LDFLAGS := -ldflags "-X pwstore/internal/cli.Version=$(VERSION)"

.PHONY: build test vet fmt install clean

build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/pwstore

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

install:
	go install $(LDFLAGS) ./cmd/pwstore

clean:
	rm -rf bin
