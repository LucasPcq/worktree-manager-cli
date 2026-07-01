BINARY   := wtm
BUILD_DIR := bin

.PHONY: build test vet lint tidy docs release install clean

build:
	go build -o $(BUILD_DIR)/$(BINARY) .

test:
	go test ./... -race -count=1

vet:
	go vet ./...

lint:
	staticcheck ./...

tidy:
	go mod tidy

docs:
	go run ./tools/gendocs

release: docs
	goreleaser release --snapshot --clean

install:
	go install .

clean:
	rm -rf $(BUILD_DIR) dist
