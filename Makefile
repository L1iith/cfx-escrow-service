.PHONY: build test vet check

build:
	mkdir -p bin
	go build -trimpath -o bin/cfx-escrow-service ./cmd/escrowd

test:
	go test ./...

vet:
	go vet ./...

check: test vet
