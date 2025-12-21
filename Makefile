# Makefile

# --- Variables ---
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOLINT=golangci-lint run
GO_SRC_DIR=src/go
TS_SRC_DIR=src/typescript

# --- Phony Targets ---
.PHONY: all clean build test lint build-go test-go lint-go clean-go build-ts test-ts lint-ts typecheck-ts

all: build test

# --- Go Targets ---
build-go:
	@echo "Building Go services..."
	cd $(GO_SRC_DIR) && $(GOBUILD) -v ./...

test-go:
	@echo "Testing Go services..."
	cd $(GO_SRC_DIR) && $(GOTEST) -v ./...

lint-go:
	@echo "Linting Go..."
	cd $(GO_SRC_DIR) && go vet ./...

clean-go:
	@echo "Cleaning Go..."
	cd $(GO_SRC_DIR) && $(GOCLEAN)

# --- TypeScript Targets ---
# Assuming one package.json per function for now, or a root workspace.
# Let's assume we iterate over directories in src/typescript

TS_DIRS := $(shell find $(TS_SRC_DIR) -mindepth 1 -maxdepth 1 -type d)

build-ts:
	@echo "Building TypeScript services..."
	@for dir in $(TS_DIRS); do \
		if [ -f "$$dir/package.json" ]; then \
			echo "Building $$dir..."; \
			(cd $$dir && npm install && npm run build); \
		fi \
	done

test-ts:
	@echo "Testing TypeScript services..."
	@for dir in $(TS_DIRS); do \
		if [ -f "$$dir/package.json" ]; then \
			echo "Testing $$dir..."; \
			(cd $$dir && npm test); \
		fi \
	done

lint-ts:
	@echo "Linting TypeScript..."
	@for dir in $(TS_DIRS); do \
		if [ -f "$$dir/package.json" ]; then \
			echo "Linting $$dir..."; \
			(cd $$dir && npm run lint); \
		fi \
	done

typecheck-ts:
	@echo "Typechecking TypeScript..."
	@for dir in $(TS_DIRS); do \
		if [ -f "$$dir/package.json" ]; then \
			echo "Typechecking $$dir..."; \
			(cd $$dir && npx tsc --noEmit); \
		fi \
	done

# --- Combined Targets ---
build: build-go build-ts
test: test-go test-ts
lint: lint-go lint-ts
clean: clean-go
	rm -rf bin/
