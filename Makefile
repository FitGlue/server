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

setup:
	@echo "Setting up dependencies..."
	@echo "Installing Go dependencies..."
	cd $(GO_SRC_DIR) && $(GOCMD) mod download
	@echo "Installing TypeScript dependencies..."
	cd $(TS_SRC_DIR) && npm install
	@echo "Setup complete."

generate:
	@echo "Generating Protocol Buffers..."
	# Generate Go
	protoc --go_out=$(GO_SRC_DIR)/pkg/types/pb --go_opt=paths=source_relative \
		--proto_path=src/proto src/proto/*.proto
	# Generate TypeScript (requires ts-proto installed)
	cd $(TS_SRC_DIR) && npx protoc --plugin=./node_modules/.bin/protoc-gen-ts_proto \
		--ts_proto_out=shared/src/types/pb --ts_proto_opt=outputEncodeMethods=false,outputJsonMethods=false,outputClientImpl=false \
		--proto_path=../proto ../proto/*.proto
	# Generate OpenAPI Clients
	@echo "Generating OpenAPI Clients..."
	@for dir in src/openapi/*; do \
		if [ -d "$$dir" ]; then \
			SERVICE=$$(basename $$dir); \
			echo "Processing $$SERVICE..."; \
			\
			# TypeScript Generation \
			echo "  [TS] Generating schema.ts for $$SERVICE..."; \
			mkdir -p $(TS_SRC_DIR)/shared/src/integrations/$${SERVICE}; \
			cd $(TS_SRC_DIR)/shared && npx openapi-typescript ../../../$$dir/swagger.json -o src/integrations/$${SERVICE}/schema.ts; \
			cd ../../..; \
			\
			# Go Generation \
			echo "  [GO] Generating client for $$SERVICE..."; \
			mkdir -p $(GO_SRC_DIR)/pkg/integrations/$$SERVICE; \
			oapi-codegen -package $$SERVICE -generate types,client \
				-o $(GO_SRC_DIR)/pkg/integrations/$$SERVICE/client.gen.go \
				$$dir/swagger.json; \
		fi \
	done

# --- Go Targets ---
build-go:
	@echo "Building Go services..."
	cd $(GO_SRC_DIR) && $(GOBUILD) -v ./...

test-go:
	@echo "Testing Go services..."
	cd $(GO_SRC_DIR) && $(GOTEST) -v ./...

lint-go:
	@echo "Linting Go..."
	@echo "Checking formatting..."
	@cd $(GO_SRC_DIR) && test -z "$$(gofmt -l .)" || (echo "Go files need formatting. Run 'gofmt -w .'" && exit 1)
	@echo "Running go vet..."
	cd $(GO_SRC_DIR) && go vet ./...
	@echo "Checking for Protobuf JSON misuse..."
	@./scripts/lint-proto-json.sh

prepare-go:
	@echo "Preparing Go services..."
	python3 scripts/build_function_zips.py 2>&1

clean-go:
	@echo "Cleaning Go..."
	cd $(GO_SRC_DIR) && $(GOCLEAN)

# --- TypeScript Targets ---
# Assuming one package.json per function for now, or a root workspace.
# Let's assume we iterate over directories in src/typescript

TS_DIRS := $(shell find $(TS_SRC_DIR) -mindepth 1 -maxdepth 1 -type d)

build-ts:
	@echo "Building TypeScript services..."
	@echo "Building shared library first..."
	@(cd $(TS_SRC_DIR)/shared && npm run build) || exit 1
	@echo "Building function packages..."
	@for dir in $(TS_DIRS); do \
		if [ -f "$$dir/package.json" ] && [ "$$(basename $$dir)" != "shared" ]; then \
			echo "Building $$dir..."; \
			(cd $$dir && npm run build) || exit 1; \
		fi \
	done

test-ts:
	@echo "Testing TypeScript services..."
	@for dir in $(TS_DIRS); do \
		if [ -f "$$dir/package.json" ]; then \
			echo "Testing $$dir..."; \
			(cd $$dir && npm test) || exit 1; \
		fi \
	done

lint-ts:
	@echo "Linting TypeScript..."
	@for dir in $(TS_DIRS); do \
		if [ -f "$$dir/package.json" ]; then \
			echo "Linting $$dir..."; \
			(cd $$dir && npm run lint) || exit 1; \
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
prepare: prepare-go
clean: clean-go
	rm -rf bin/
