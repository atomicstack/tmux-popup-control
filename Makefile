GOCACHE := $(CURDIR)/.gocache
GOMODCACHE := $(CURDIR)/.gomodcache
GO_ENV := GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) GOFLAGS=-modcacherw

.PHONY: build run tidy fmt test clean-cache ensure-dirs

ensure-dirs:
	@mkdir -p $(GOCACHE) $(GOMODCACHE)

build: ensure-dirs
	$(GO_ENV) go build ./...

run: ensure-dirs
	$(GO_ENV) go run .

fmt:
	$(GO_ENV) gofmt -w .

tidy: ensure-dirs
	$(GO_ENV) go mod tidy

clean-cache:
	rm -rf $(GOCACHE) $(GOMODCACHE)

