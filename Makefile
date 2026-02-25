GO ?= go
BINARY := tmux-popup-control
GOCACHE := $(CURDIR)/.gocache
GOMODCACHE := $(CURDIR)/.gomodcache
GO_ENV := GOTMUXCC_TRACE=1 GOTMUXCC_TRACE_FILE=$(CURDIR)/gotmuxcc_trace.log GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) GOFLAGS=-modcacherw GOPROXY=off

.SILENT:

.PHONY: build run tidy fmt test clean-cache ensure-dirs cover update-gotmuxcc

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

GO_ENV_ONLINE := GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) GOFLAGS=-modcacherw GOPROXY=direct

update-gotmuxcc: ensure-dirs
	$(GO_ENV_ONLINE) go get github.com/atomicstack/gotmuxcc@main
	$(GO_ENV_ONLINE) go mod tidy
	$(GO_ENV_ONLINE) go mod vendor

clean-cache:
	rm -rf $(GOCACHE) $(GOMODCACHE)

cover:
	@echo "==> Generating coverage report"
	$(GO) test ./... -coverprofile=coverage.out
	@echo "Coverage summary:"
	$(GO) tool cover -func=coverage.out
