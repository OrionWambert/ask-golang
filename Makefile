VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X oriowambert.com/ask-golang/cmd.version=$(VERSION)

.PHONY: build test install uninstall clean

build:
	go build -ldflags "$(LDFLAGS)" -o ask$(shell go env GOEXE) .

test:
	go vet ./...
	go test ./...

install:
	sh scripts/install.sh

uninstall:
	sh scripts/uninstall.sh

clean:
	rm -f ask ask.exe
