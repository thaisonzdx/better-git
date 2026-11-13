BIN := bin/git
VERSION := $(shell git describe --tags --always --dirty)

.PHONY: build vet check clean release

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BIN) .

vet:
	go vet ./...

# Smoke test: the wrapper should hand off to the original git.
check: vet build
	$(BIN) --version

# Tag HEAD with the next YY.MM.x version; push the tag to publish.
release:
	@prefix=$$(date +%y.%m); \
	last=$$(git tag -l "$$prefix.*" --sort=-v:refname | head -1); \
	if [ -z "$$last" ]; then n=0; else n=$$(( $${last##*.} + 1 )); fi; \
	git tag "$$prefix.$$n" && echo "tagged $$prefix.$$n"

clean:
	rm -rf bin
