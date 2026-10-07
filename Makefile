VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  = -s -w -X main.version=$(VERSION)

itop:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o itop .

test:
	go test ./...

dist:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/itop-linux-amd64 .
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/itop-linux-arm64 .
	cd dist && sha256sum itop-* > sha256sums.txt

clean:
	rm -rf itop dist

.PHONY: itop test dist clean
