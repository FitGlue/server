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

all: generate-proto clean inject-shared build-ts build-go

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
	# Source: shared/typescript/src/* -> Target: functions/DIR/src/shared/
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
		rsync -av --exclude='node_modules' --exclude='*_test.go' --exclude='go.mod' --exclude='go.sum' shared/go/ $(FUNCTIONS_DIR)/$$func/pkg/shared/; \
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
		(cd $(FUNCTIONS_DIR)/$$func && go mod tidy && go build -v ./...); \
	done

local:
	@./scripts/local_run.sh
