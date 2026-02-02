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
.PHONY: all clean build test lint build-go test-go lint-go clean-go build-ts test-ts lint-ts typecheck-ts clean-ts plugin-source plugin-enricher plugin-destination lint-codebase lint-shared-modules tools build-tools-go build-tools-ts prepare prepare-go prepare-ts

all: generate clean lint build test


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
	cd $(TS_SRC_DIR) && protoc --plugin=./node_modules/.bin/protoc-gen-ts_proto \
		--experimental_allow_proto3_optional \
		--ts_proto_out=shared/src/types/pb --ts_proto_opt=outputEncodeMethods=false,outputJsonMethods=false,outputClientImpl=false,useOptionals=messages \
		--proto_path=../proto ../proto/*.proto
	# Generate OpenAPI Clients
	@echo "Generating OpenAPI Clients..."
	@for dir in src/openapi/*; do \
		if [ -d "$$dir" ]; then \
			SERVICE=$$(basename $$dir); \
			echo "Processing $$SERVICE..."; \
			echo "  [TS] Generating schema.ts for $$SERVICE..."; \
			mkdir -p $(TS_SRC_DIR)/shared/src/integrations/$${SERVICE}; \
			cd $(TS_SRC_DIR)/shared && npx openapi-typescript ../../../$$dir/swagger.json -o src/integrations/$${SERVICE}/schema.ts; \
			cd ../../..; \
			echo "  [GO] Generating client for $$SERVICE..."; \
			mkdir -p $(GO_SRC_DIR)/pkg/integrations/$$SERVICE; \
			oapi-codegen -package $$SERVICE -generate types,client \
				-o $(GO_SRC_DIR)/pkg/integrations/$$SERVICE/client.gen.go \
				$$dir/swagger.json; \
		fi \
	done
	# Generate enum formatters (TS + Go)
	@echo "Generating enum formatters..."
	@npx ts-node scripts/generate-enum-formatters.ts
	# Copy all generated types to web (if exists)
	@if [ -d "../web" ]; then \
		echo "Copying generated types to web..."; \
		mkdir -p ../web/src/types/pb; \
		cp src/typescript/shared/src/types/pb/*.ts ../web/src/types/pb/; \
		echo "Web types updated at ../web/src/types/pb/"; \
	else \
		echo "Skipping web types (../web not found)"; \
	fi

# --- Go Targets ---
build-go:
	@echo "Building Go services..."
	cd $(GO_SRC_DIR) && $(GOBUILD) -v ./...

build-tools-go:
	@echo "Building Go tools..."
	@mkdir -p bin
	@echo "  Building fit-gen tool..."
	cd $(GO_SRC_DIR) && $(GOBUILD) -o ../../bin/fit-gen ./cmd/fit-gen
	@echo "  Building fit-inspect tool..."
	cd $(GO_SRC_DIR) && $(GOBUILD) -o ../../bin/fit-inspect ./cmd/fit-inspect

test-go:
	@echo "Testing Go services..."
	cd $(GO_SRC_DIR) && $(GOTEST) -v ./...

lint-go:
	@echo "Linting Go..."
	@echo "Checking formatting..."
	@cd $(GO_SRC_DIR) && test -z "$$(gofmt -l .)" || (echo "Go files need formatting. Run 'gofmt -w .'" && exit 1)
	@echo "Running go vet (excluding generated clients)..."
	@cd $(GO_SRC_DIR) && go vet $$(go list ./... | grep -v '/integrations/')
	@echo "Checking for Protobuf JSON misuse..."
	@./scripts/lint-proto-json.sh

prepare-go:
	@echo "Preparing Go function ZIPs..."
	python3 scripts/build_function_zips.py 2>&1

prepare-ts:
	@echo "Preparing TypeScript function ZIPs..."
	python3 scripts/build_typescript_zips.py 2>&1

# Parallel prepare - Go and TS ZIPs can be built concurrently
prepare:
	@$(MAKE) -j2 prepare-go prepare-ts

clean-go:
	@echo "Cleaning Go..."
	cd $(GO_SRC_DIR) && $(GOCLEAN)

# --- TypeScript Targets ---
# Assuming one package.json per function for now, or a root workspace.
# Let's assume we iterate over directories in src/typescript

TS_DIRS := $(shell find $(TS_SRC_DIR) -mindepth 1 -maxdepth 1 -type d -not -name node_modules)

# Note: We enforce building 'shared' first because other packages depend on it.
# Then we build all other workspaces in parallel for speed.
TS_HANDLER_DIRS := $(shell find $(TS_SRC_DIR) -mindepth 1 -maxdepth 1 -type d -not -name node_modules -not -name shared -not -name mcp-server -not -name admin-cli -not -name parkrun-fetcher)
TS_TOOL_DIRS := $(TS_SRC_DIR)/mcp-server $(TS_SRC_DIR)/admin-cli

TS_HANDLER_NAMES := $(notdir $(TS_HANDLER_DIRS))
TS_TOOL_NAMES := $(notdir $(TS_TOOL_DIRS))

# Helper target to build the shared library first
build-shared:
	@echo "Building shared library..."
	@cd $(TS_SRC_DIR) && npm run build --workspace=@fitglue/shared

# Pattern rule for building any typescript workspace
build-handler-%: build-shared
	@echo "Building handler $*..."
	@cd $(TS_SRC_DIR) && npm run build --workspace=$* --if-present

build-tool-%: build-shared
	@echo "Building tool $*..."
	@cd $(TS_SRC_DIR) && npm run build --workspace=$* --if-present

# Build all handlers using Make's job server
build-ts: build-shared $(addprefix build-handler-,$(TS_HANDLER_NAMES))
	@echo "TypeScript service builds complete."

# Build all tools using Make's job server
build-tools-ts: build-shared $(addprefix build-tool-,$(TS_TOOL_NAMES))
	@echo "TypeScript tools build complete."

tools: build-tools-ts build-tools-go

# Pattern rule for testing any typescript workspace
test-handler-%: build-shared
	@echo "Testing handler $*..."
	@cd $(TS_SRC_DIR) && npm test --workspace=$* --if-present

# Test all handlers using Make's job server
test-ts: build-shared $(addprefix test-handler-,$(TS_HANDLER_NAMES))
	@echo "TypeScript tests complete."

# Pattern rule for linting any typescript workspace
lint-handler-%:
	@echo "Linting handler $*..."
	@cd $(TS_SRC_DIR) && npm run lint --workspace=$* --if-present

# Lint all handlers using Make's job server
lint-ts: $(addprefix lint-handler-,$(TS_HANDLER_NAMES))
	@echo "TypeScript linting complete."

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
# P1: Parallel builds - Go and TS can build concurrently
build:
	@$(MAKE) build-go
	@$(MAKE) -j4 build-ts

# P2: Parallel tests - Go and TS tests can run concurrently
test:
	@$(MAKE) test-go
	@$(MAKE) -j4 test-ts

# P3: Parallel lint - Go, TS, and codebase checks can run concurrently
lint:
	@$(MAKE) -j4 lint-go lint-ts lint-codebase lint-shared-modules

# P4: Parallel clean
clean:
	@$(MAKE) -j2 clean-go clean-ts
	rm -rf bin/

# --- Codebase Consistency Check ---
lint-codebase:
	@echo "Running codebase consistency checks..."
	@npm install --silent
	@npx ts-node scripts/lint-codebase.ts

lint-verbose:
	@echo "Running codebase consistency checks (verbose)..."
	@npm install --silent
	@npx ts-node scripts/lint-codebase.ts --verbose

# --- Shared Modules Validation ---
# Ensures shared_modules.json stays in sync with the codebase
lint-shared-modules:
	@echo "Validating shared_modules.json..."
	@python3 scripts/validate_shared_modules.py


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
