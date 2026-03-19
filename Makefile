BINARY = tweaker

.PHONY: build clean

build:
	CGO_ENABLED=0 go build -ldflags "-s -w" -o bin/$(BINARY) ./cmd/myserver

clean:
	rm -rf bin/
