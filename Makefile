.PHONY: all clean inject-shared build-ts build-go generate-proto

# Directories
SHARED_DIR := shared
PROTO_DIR := shared/proto
FUNCTIONS_DIR := functions

# Target Functions
TS_FUNCTIONS := hevy-handler keiser-poller
GO_FUNCTIONS := enricher router strava-uploader

# Protoc
# Protoc
PROTOC := $(shell if [ -x "./bin/protoc" ]; then echo "./bin/protoc"; else which protoc; fi)
PROTOC_GEN_GO := $(shell go env GOPATH)/bin/protoc-gen-go
TS_PROTO_PLUGIN := ./node_modules/.bin/protoc-gen-ts_proto

all: generate-proto clean inject-shared build-ts build-go prepare-shared-ts

setup:
	@echo "Setting up fresh environment..."
	npm install
	$(MAKE) all
	@echo "Setup complete! Run 'make test' to verify or 'make local' to start."

build: all

generate-proto:
	@echo "Generating Protobuf code..."
	mkdir -p shared/types/pb
	mkdir -p shared/go/types/pb

	# Generate Go (Output to shared/go/types/pb)
	$(PROTOC) --plugin=protoc-gen-go=$(PROTOC_GEN_GO) \
		--go_out=shared/go/types/pb --go_opt=paths=source_relative \
		--proto_path=$(SHARED_DIR) \
		$(PROTO_DIR)/activity.proto
	# Generate TypeScript (Output to shared/typescript/src/types/pb)
	$(PROTOC) --plugin=protoc-gen-ts_proto=$(TS_PROTO_PLUGIN) \
		--ts_proto_out=shared/types/pb \
		--ts_proto_opt=esModuleInterop=true \
		--proto_path=$(SHARED_DIR) \
		$(PROTO_DIR)/activity.proto
	# Copy generated types to their respective language folders
	# We want to preserve 'proto' subdirectory if it exists in the output
	mkdir -p shared/typescript/src/types/pb
	cp -r shared/types/pb/* shared/typescript/src/types/pb/ || true

inject-shared:
	@echo "Cleaning shared injections..."
	@rm -rf functions/*/src/shared
	@rm -rf functions/*/pkg/shared
	@echo "Injecting shared code..."
	# TypeScript Injection
	# Source: shared/typescript/src/* -> Target: functions/DIR/src/shared/ (Includes framework)
	@for func in $(TS_FUNCTIONS); do \
		echo "Injected TS into $$func"; \
		mkdir -p $(FUNCTIONS_DIR)/$$func/src/shared; \
		rsync -av --exclude='node_modules' --exclude='*.test.ts' shared/typescript/src/ $(FUNCTIONS_DIR)/$$func/src/shared/; \
	done
	# Go Injection
	# Source: shared/go/* -> Target: functions/DIR/pkg/shared/
	@for func in $(GO_FUNCTIONS); do \
		echo "Injected Go into $$func"; \
		mkdir -p $(FUNCTIONS_DIR)/$$func/pkg/shared; \
		rsync -av --exclude='node_modules' --exclude='*_test.go' shared/go/ $(FUNCTIONS_DIR)/$$func/pkg/shared/; \
		mkdir -p $(FUNCTIONS_DIR)/$$func/pkg/shared/proto; \
		cp -r shared/proto/* $(FUNCTIONS_DIR)/$$func/pkg/shared/proto/; \
	done

clean:
	@echo "Cleaning shared injections..."
	@rm -rf functions/*/src/shared
	@rm -rf functions/*/pkg/shared

build-ts:
	@echo "Building TypeScript functions..."
	@for func in $(TS_FUNCTIONS); do \
		(cd $(FUNCTIONS_DIR)/$$func && npm install && npm run build); \
	done

build-go:
	@echo "Building Go functions..."
	@for func in $(GO_FUNCTIONS); do \
		(cd $(FUNCTIONS_DIR)/$$func && go mod tidy && GOWORK=off go build -v ./...); \
	done

prepare-shared-ts:
	@echo "Preparing Shared TypeScript..."
	@(cd shared/typescript && npm install)

local: inject-shared
	@./scripts/local_run.sh

# --- Unified Operations ---

lint:
	@echo "Linting Go..."
	@for func in $(GO_FUNCTIONS); do \
		echo "Linting $$func..."; \
		(cd $(FUNCTIONS_DIR)/$$func && GOWORK=off go vet ./...); \
	done
	@(cd shared/go && GOWORK=off go vet ./...)
	@echo "Linting TypeScript (Type Check)..."
	@for func in $(TS_FUNCTIONS); do \
		echo "Linting $$func..."; \
		(cd $(FUNCTIONS_DIR)/$$func && npm run build); \
	done
	@(cd shared/typescript && npm run build)

test:
	@echo "Running all tests..."
	@echo ">> Go Tests"
	@for func in $(GO_FUNCTIONS); do \
		echo "Testing $$func..."; \
		(cd $(FUNCTIONS_DIR)/$$func && go test ./...); \
	done
	@echo ">> Go Tests (Shared)"
	@(cd shared/go && go test ./...)
	@echo ">> TypeScript Tests (Hevy)"
	@(cd functions/hevy-handler && npm test)
	@echo ">> TypeScript Tests (Keiser)"
	@(cd functions/keiser-poller && npm test)
	@echo ">> TypeScript Tests (Shared)"
	@(cd shared/typescript && npm test)

deploy-dev:
	@echo "Deploying to Dev (fitglue-server-dev)..."
	@terraform -chdir=terraform apply -auto-approve -var="project_id=fitglue-server-dev"

verify-dev:
	@echo "Verifying Dev Environment..."
	@TARGET_URL="https://hevy-webhook-handler-56cqxmt5jq-uc.a.run.app" npx ts-node scripts/verify_cloud.ts

deploy-test:
	@echo "Deploying to Test (fitglue-server-test)..."
	@terraform -chdir=terraform apply -auto-approve -var="project_id=fitglue-server-test"

verify-test:
	@echo "Verifying Test Environment..."
	@echo "NOTE: Set TARGET_URL env var to the Test function URL"
	@if [ -z "$$TARGET_URL" ]; then echo "Error: TARGET_URL not set"; exit 1; fi
	@npx ts-node scripts/verify_cloud.ts

deploy-prod:
	@echo "Deploying to Prod (fitglue-server-prod)..."
	@terraform -chdir=terraform apply -auto-approve -var="project_id=fitglue-server-prod"

verify-prod:
	@echo "Verifying Prod Environment..."
	@echo "NOTE: Set TARGET_URL env var to the Prod function URL"
	@if [ -z "$$TARGET_URL" ]; then echo "Error: TARGET_URL not set"; exit 1; fi
	@npx ts-node scripts/verify_cloud.ts
