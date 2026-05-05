GO ?= go
BINARY := tmux-popup-control
GOCACHE := $(CURDIR)/.gocache
GOMODCACHE := $(CURDIR)/.gomodcache
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags="-X main.Version=$(VERSION)"
# Base env for offline builds. GOTMUXCC_TRACE is intentionally NOT set here:
# enabling it makes every gotmuxcc call write a trace line to disk and slows
# integration tests by ~4x (6s → 24s for `make test`). Use `make test-trace`
# (or set GOTMUXCC_TRACE=1 manually) when you actually want the trace log.
GO_ENV := GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) GOFLAGS=-modcacherw GOPROXY=off
GO_ENV_TRACED := GOTMUXCC_TRACE=1 GOTMUXCC_TRACE_FILE=$(CURDIR)/gotmuxcc_trace.log $(GO_ENV)

.SILENT:

.PHONY: build run tidy fmt test test-trace clean-cache ensure-dirs cover update-gotmuxcc release

ensure-dirs:
	mkdir -p $(GOCACHE) $(GOMODCACHE)

build: ensure-dirs
	$(GO_ENV) go build $(LDFLAGS) -o $(BINARY) .

run: ensure-dirs
	$(GO_ENV) go run .

fmt:
	$(GO_ENV) gofmt -w .

tidy: ensure-dirs
	$(GO_ENV) go mod tidy

test: ensure-dirs
	# -p 4 caps concurrent package execution at 4. The default
	# (GOMAXPROCS=16 on this machine) runs every package in parallel,
	# which means the 4 integration test binaries (resurrect, testutil,
	# tmux, ui) each fire their tests at full thread pressure
	# simultaneously — every tmux operation then queues behind every
	# other test's tmux operations and a clean run stretches from ~7s
	# to ~24s. With -p 4 the integration packages still run
	# concurrently with each other but the rest of the suite fills
	# remaining slots one-at-a-time, keeping per-test contention low.
	# Cold cache: ~7s. Warm cache (no source changes): <1s.
	$(GO_ENV) go test -p 4 ./...

test-trace: ensure-dirs
	# Same as `make test` but with gotmuxcc command tracing enabled.
	# Trace lines land in $(CURDIR)/gotmuxcc_trace.log. Roughly 4x
	# slower than `make test`; use only when you need the trace.
	$(GO_ENV_TRACED) go test -p 4 ./...

GO_ENV_ONLINE := GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) GOFLAGS=-modcacherw GOPROXY=direct

update-gotmuxcc: ensure-dirs
	$(GO_ENV_ONLINE) go get github.com/atomicstack/gotmuxcc@latest
	$(GO_ENV_ONLINE) go mod tidy
	$(GO_ENV_ONLINE) go mod vendor

clean-cache:
	rm -rf $(GOCACHE) $(GOMODCACHE)

cover:
	@echo "==> Generating coverage report"
	$(GO) test ./... -coverprofile=coverage.out
	@echo "Coverage summary:"
	$(GO) tool cover -func=coverage.out

RELEASE_DIR := dist
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64
RELEASE_SUPPORT_FILES := README.md main.sh main.tmux

release: ensure-dirs
	rm -rf $(RELEASE_DIR)
	mkdir -p $(RELEASE_DIR)
	$(foreach platform,$(PLATFORMS),\
		$(eval GOOS := $(word 1,$(subst /, ,$(platform))))\
		$(eval GOARCH := $(word 2,$(subst /, ,$(platform))))\
		echo "Building $(GOOS)/$(GOARCH)..." && \
		$(GO_ENV) GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 \
			go build $(LDFLAGS) -o $(RELEASE_DIR)/$(BINARY)-$(GOOS)-$(GOARCH) . && \
	) true
	chmod +x $(RELEASE_DIR)/$(BINARY)-*
	cd $(RELEASE_DIR) && for f in $(BINARY)-*; do \
		stage_dir="$$(mktemp -d ./release.XXXXXX)"; \
		cp "$$f" "$$stage_dir/$(BINARY)"; \
		cp $(addprefix ../,$(RELEASE_SUPPORT_FILES)) "$$stage_dir/"; \
		chmod +x "$$stage_dir/$(BINARY)" "$$stage_dir/main.sh" "$$stage_dir/main.tmux"; \
		COPYFILE_DISABLE=1 tar --no-xattrs --no-mac-metadata -czf "$$f.tar.gz" -C "$$stage_dir" .; \
		rm -rf "$$stage_dir" "$$f"; \
	done
	cd $(RELEASE_DIR) && shasum -a 256 *.tar.gz > checksums.txt
	gh release create v$(VERSION) $(RELEASE_DIR)/* \
		--title "v$(VERSION)" \
		--generate-notes
