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
.PHONY: all clean build test lint build-go test-go lint-go clean-go build-ts test-ts lint-ts typecheck-ts clean-ts plugin-source plugin-enricher plugin-destination lint-codebase

all: generate build test lint


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
		--experimental_allow_proto3_optional \
		--proto_path=src/proto src/proto/*.proto
	# Generate TypeScript (requires ts-proto installed)
	cd $(TS_SRC_DIR) && npx protoc --plugin=./node_modules/.bin/protoc-gen-ts_proto \
		--ts_proto_out=shared/src/types/pb --ts_proto_opt=outputEncodeMethods=false,outputJsonMethods=false,outputClientImpl=false,useOptionals=messages \
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
build-go: clean-go
	@echo "Building Go services..."
	cd $(GO_SRC_DIR) && $(GOBUILD) -v ./...
	@echo "Building fit-gen tool..."
	cd $(GO_SRC_DIR) && $(GOBUILD) -o ../../bin/fit-gen ./cmd/fit-gen
	@echo "Building fit-inspect tool..."
	cd $(GO_SRC_DIR) && $(GOBUILD) -o ../../bin/fit-inspect ./cmd/fit-inspect

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

TS_DIRS := $(shell find $(TS_SRC_DIR) -mindepth 1 -maxdepth 1 -type d -not -name node_modules)

# Note: We enforce building 'shared' first because other packages depend on it
# and standard npm workspaces don't guarantee topological build order for scripts.
build-ts: clean-ts
	@echo "Building TypeScript services (via Workspaces)..."
	@echo "Building shared library..."
	@cd $(TS_SRC_DIR) && npm run build --workspace=@fitglue/shared
	@echo "Building all workspaces..."
	@cd $(TS_SRC_DIR) && npm run build --workspaces --if-present

test-ts:
	@echo "Testing TypeScript services..."
	@cd $(TS_SRC_DIR) && npm test --workspaces --if-present

lint-ts:
	@echo "Linting TypeScript..."
	@cd $(TS_SRC_DIR) && npm run lint --workspaces --if-present

typecheck-ts:
	@echo "Typechecking TypeScript..."
	@# tsc --build might be better if tsconfig references are set up, but iterating is safe for now via npm
	@cd $(TS_SRC_DIR) && npm exec --workspaces --if-present -- tsc --noEmit

clean-ts:
	@echo "Cleaning TypeScript..."
	@# We can't easily use workspaces for cleaning specific dirs without a script,
	@# but we can just ask every workspace to run its clean script if it exists?
	@# Most don't have a 'clean' script. The previous logic was reliable.
	@# Let's keep the find logic for cleaning as it's robust against missing scripts.
	@for dir in $(TS_DIRS); do \
		if [ -f "$$dir/package.json" ]; then \
			echo "Cleaning $$dir..."; \
			rm -rf $$dir/dist $$dir/build; \
		fi \
	done

# --- Combined Targets ---
build: build-go build-ts
test: test-go test-ts
lint: lint-go lint-ts lint-codebase
prepare: prepare-go
clean: clean-go clean-ts
	rm -rf bin/

# --- Codebase Consistency Check ---
lint-codebase:
	@echo "Running codebase consistency checks..."
	@npm install --silent
	@npx ts-node scripts/lint-codebase.ts


# --- Plugin Scaffolding ---
# Usage: make plugin-source name=garmin
#        make plugin-enricher name=weather
#        make plugin-destination name=runkeeper
plugin-source:
ifndef name
	$(error Usage: make plugin-source name=<name>)
endif
	./scripts/new-plugin.sh source $(name)

plugin-enricher:
ifndef name
	$(error Usage: make plugin-enricher name=<name>)
endif
	./scripts/new-plugin.sh enricher $(name)

plugin-destination:
ifndef name
	$(error Usage: make plugin-destination name=<name>)
endif
	./scripts/new-plugin.sh destination $(name)
