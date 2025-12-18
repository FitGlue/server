.PHONY: all clean inject-shared build-ts build-go generate-proto

# Directories
SHARED_DIR := shared
PROTO_DIR := shared/proto
FUNCTIONS_DIR := functions

# Target Functions
TS_FUNCTIONS := hevy-handler keiser-poller
GO_FUNCTIONS := enricher router strava-uploader

# Protoc
PROTOC := ./bin/protoc
PROTOC_GEN_GO := $(shell go env GOPATH)/bin/protoc-gen-go
TS_PROTO_PLUGIN := ./node_modules/.bin/protoc-gen-ts_proto

all: generate-proto clean inject-shared build-ts build-go

generate-proto:
	@echo "Generating Protobuf code..."
	mkdir -p shared/types/pb
	# Generate Go
	$(PROTOC) --plugin=protoc-gen-go=$(PROTOC_GEN_GO) \
		--go_out=shared/types/pb --go_opt=paths=source_relative \
		--proto_path=$(SHARED_DIR) \
		$(PROTO_DIR)/activity.proto
	# Generate TypeScript
	$(PROTOC) --plugin=protoc-gen-ts_proto=$(TS_PROTO_PLUGIN) \
		--ts_proto_out=shared/types/pb \
		--ts_proto_opt=esModuleInterop=true \
		--proto_path=$(SHARED_DIR) \
		$(PROTO_DIR)/activity.proto

inject-shared:
	@echo "Injecting shared code..."
	# TypeScript Injection (Only .ts files)
	@for func in $(TS_FUNCTIONS); do \
		mkdir -p $(FUNCTIONS_DIR)/$$func/src/shared; \
		cd $(SHARED_DIR) && find . -name "*.ts" -exec cp --parents {} ../$(FUNCTIONS_DIR)/$$func/src/shared/ \; ; \
		echo "Injected TS into $$func"; \
	done
	# Go Injection (Only .go and .proto files)
	@for func in $(GO_FUNCTIONS); do \
		mkdir -p $(FUNCTIONS_DIR)/$$func/pkg/shared; \
		cd $(SHARED_DIR) && find . \( -name "*.go" -o -name "*.proto" \) -exec cp --parents {} ../$(FUNCTIONS_DIR)/$$func/pkg/shared/ \; ; \
		echo "Injected Go into $$func"; \
	done

clean:
	@echo "Cleaning shared injections..."
	@for func in $(TS_FUNCTIONS); do \
		rm -rf $(FUNCTIONS_DIR)/$$func/src/shared; \
	done
	@for func in $(GO_FUNCTIONS); do \
		rm -rf $(FUNCTIONS_DIR)/$$func/pkg/shared; \
	done

build-ts:
	@echo "Building TypeScript functions..."
	@for func in $(TS_FUNCTIONS); do \
		cd $(FUNCTIONS_DIR)/$$func && npm install && npm run build; \
	done

build-go:
	@echo "Building Go functions..."
	@for func in $(GO_FUNCTIONS); do \
		cd $(FUNCTIONS_DIR)/$$func && go mod tidy && go build -v ./...; \
	done
