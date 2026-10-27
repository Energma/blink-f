.PHONY: build rmx test test-v lint clean install run

BINARY=blink
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

# rmx (github.com/Energma/rust-terminal-multiplexer) is built from a sibling
# checkout and embedded into blink, so the result needs no tmux on the box.
RMX_SRC?=../rust-mux
RMX_BIN=internal/rmxbin/bin/rmx

build: rmx
	go build $(LDFLAGS) -o $(BINARY) ./cmd/blink/

rmx:
	cargo build --release --manifest-path $(RMX_SRC)/Cargo.toml
	cp $(RMX_SRC)/target/release/rmx $(RMX_BIN)

test:
	go test ./...

test-v:
	go test -v ./...

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY) $(RMX_BIN)

install: build
	cp $(BINARY) $(GOPATH)/bin/

run:
	go run ./cmd/blink/
