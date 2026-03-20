BINARY  = tweaker
VERSION = $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -s -w -X main.version=$(VERSION)
GOFLAGS = CGO_ENABLED=0

PLATFORMS = linux/amd64 linux/arm64 windows/amd64 darwin/amd64 darwin/arm64

.PHONY: build clean release release-all

build:
	$(GOFLAGS) go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/myserver

release-all: clean
	@mkdir -p bin
	@$(foreach platform,$(PLATFORMS), \
		$(eval OS   = $(word 1,$(subst /, ,$(platform)))) \
		$(eval ARCH = $(word 2,$(subst /, ,$(platform)))) \
		$(eval EXT  = $(if $(filter windows,$(OS)),.exe,)) \
		$(eval OUT  = bin/$(BINARY)-$(OS)-$(ARCH)$(EXT)) \
		echo "Building $(OUT)..." && \
		$(GOFLAGS) GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags "$(LDFLAGS)" -o $(OUT) ./cmd/myserver && \
	) true

release-linux:
	@mkdir -p bin
	$(GOFLAGS) GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY)-linux-amd64 ./cmd/myserver
	$(GOFLAGS) GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY)-linux-arm64 ./cmd/myserver

clean:
	rm -rf bin/
