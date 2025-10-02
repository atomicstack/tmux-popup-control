BINARY := tmux-popup-control
GOCACHE := $(CURDIR)/.gocache
GOMODCACHE := $(CURDIR)/.gomodcache
GO_ENV := GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) GOFLAGS=-modcacherw GOPROXY=off

.SILENT:

.PHONY: build run tidy fmt test clean-cache ensure-dirs

ensure-dirs:
	mkdir -p $(GOCACHE) $(GOMODCACHE)

build: ensure-dirs
	$(GO_ENV) go build -o $(BINARY) .

run: ensure-dirs
	$(GO_ENV) go run .

fmt:
	$(GO_ENV) gofmt -w .

tidy: ensure-dirs
	$(GO_ENV) go mod tidy

test: ensure-dirs
	$(GO_ENV) go test ./...

clean-cache:
	rm -rf $(GOCACHE) $(GOMODCACHE)
