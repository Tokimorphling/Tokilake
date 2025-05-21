# Tokiame Makefile

# Support building on Linux, macOS and Windows platforms
# Includes gRPC proto file generation

# Go command
GO := go

# Project settings
PROJECT_NAME := tokiame
PACKAGE_NAME := tokiame # Used in LDFLAGS, e.g., for package path in version info
MAIN_PATH := ./cmd/tokiame
OUTPUT_DIR := ./build/bin

# Proto settings
PROTO_SRC_DIR := ../crates/inference-server/proto
# This first line will find all .proto files, but it's immediately overwritten.
PROTO_FILES := $(wildcard $(PROTO_SRC_DIR)/*.proto)
# This line means only tokilake.proto will be processed. If you want all, comment this out and uncomment the one above.
# PROTO_FILES := $(PROTO_SRC_DIR)/tokilake.proto

GENERATED_PB_DIR := ./internal/pb # Output for generated .pb.go files, as per go_package

# Detect operating system and set platform-specific commands
ifeq ($(OS),Windows_NT)
    PLATFORM := windows
    EXECUTABLE := $(PROJECT_NAME).exe
    RM := del /Q /F
    # MKDIR_CMD creates a directory if it doesn't exist. PowerShell for Windows.
    # Usage: $(call MKDIR_CMD, path/to/dir)
    define MKDIR_CMD
    PowerShell -Command "New-Item -ItemType Directory -Path '$(subst /,\,$(1))' -Force -ErrorAction SilentlyContinue"
    endef
    RMDIR := cmd /c rmdir /S /Q
    fix_path = $(subst /,\,$(1))
else
    PLATFORM := $(shell uname -s | tr '[:upper:]' '[:lower:]')
    EXECUTABLE := $(PROJECT_NAME)
    RM := rm -f
    # Usage: $(call MKDIR_CMD, path/to/dir)
    define MKDIR_CMD
    mkdir -p $(1)
    endef
    RMDIR := rm -rf
    fix_path = $(1)
endif

# Full path to the output binary
OUTPUT_BINARY := $(OUTPUT_DIR)/$(EXECUTABLE)



LDFLAGS := -ldflags="-s -w"

GOFLAGS := -trimpath



# Default target
.PHONY: all
all: build

# Generate Go code from .proto files
# This target ensures the directory for generated files exists.
.PHONY: proto
proto:
	@echo "Generating Go code from .proto files in $(PROTO_SRC_DIR)..."
	@$(call MKDIR_CMD,$(GENERATED_PB_DIR))
	@protoc --proto_path=$(PROTO_SRC_DIR) \
	    --go_out=$(GENERATED_PB_DIR)	 \
	    --go_opt=paths=source_relative 	\
	    --go-grpc_out=$(GENERATED_PB_DIR) \
	    --go-grpc_opt=paths=source_relative \
	    $(PROTO_FILES)
	@echo "Go code generation complete. Files are in $(GENERATED_PB_DIR)/"

# Create output directory if it doesn't exist
# This is a prerequisite for the build target
$(OUTPUT_DIR):
	@echo "Creating output directory: $(OUTPUT_DIR)"
	@$(call MKDIR_CMD,$(OUTPUT_DIR))

# Build target: Depends on proto files being generated and output directory existing
.PHONY: build
build: $(OUTPUT_DIR) proto
	@echo "Building $(PROJECT_NAME) for $(PLATFORM)..."
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(call fix_path,$(OUTPUT_BINARY)) $(MAIN_PATH)
	@echo "Build completed: $(call fix_path,$(OUTPUT_BINARY))"

# Run target
.PHONY: run
run: build
	@echo "Running $(PROJECT_NAME)..."
	$(call fix_path,$(OUTPUT_BINARY))

# Test target
.PHONY: test
test:
	@echo "Running tests..."
	$(GO) test ./... -v

# Format target
.PHONY: fmt
fmt:
	@echo "Formatting Go files..."
	$(GO) fmt ./...

# Install target
.PHONY: install
install: build # Ensure binary is built before installing
	@echo "Installing $(PROJECT_NAME)..."
	$(GO) install $(GOFLAGS) $(LDFLAGS) $(MAIN_PATH)
	@echo "$(PROJECT_NAME) installed to $(shell $(GO) env GOPATH)/bin or $(shell $(GO) env GOBIN)"

# Clean generated protobuf and gRPC Go files
.PHONY: clean-proto
clean-proto:
	@echo "Cleaning generated Go protobuf and gRPC files from $(GENERATED_PB_DIR)..."
	-@$(RM) $(call fix_path,$(GENERATED_PB_DIR)/*.pb.go)
	-@$(RM) $(call fix_path,$(GENERATED_PB_DIR)/*_grpc.pb.go)
	ifeq ($(PLATFORM),windows)
		@powershell -Command "if (Test-Path '$(call fix_path,$(GENERATED_PB_DIR))') { if (-not (Get-ChildItem -Path '$(call fix_path,$(GENERATED_PB_DIR))' -ErrorAction SilentlyContinue)) { Write-Host 'Removing empty directory $(call fix_path,$(GENERATED_PB_DIR))'; Remove-Item -Path '$(call fix_path,$(GENERATED_PB_DIR))' -Force -ErrorAction SilentlyContinue } }"
	else
		@if [ -d "$(GENERATED_PB_DIR)" ] && [ -z "$$(ls -A $(GENERATED_PB_DIR))" ]; then \
			echo "Removing empty directory $(GENERATED_PB_DIR)"; \
			$(RMDIR) $(GENERATED_PB_DIR); \
		fi
	endif
	@echo "Proto cleaning complete."

# Clean target: Cleans build artifacts and generated proto files
.PHONY: clean
clean: clean-proto
	@echo "Cleaning build artifacts..."
	-@$(RM) $(call fix_path,$(OUTPUT_BINARY))
	@# Optionally remove the OUTPUT_DIR if it's empty
	ifeq ($(PLATFORM),windows)
		@powershell -Command "if (Test-Path '$(call fix_path,$(OUTPUT_DIR))') { if (-not (Get-ChildItem -Path '$(call fix_path,$(OUTPUT_DIR))' -ErrorAction SilentlyContinue)) { Write-Host 'Removing empty directory $(call fix_path,$(OUTPUT_DIR))'; Remove-Item -Path '$(call fix_path,$(OUTPUT_DIR))' -Force -ErrorAction SilentlyContinue } }"
	else
		@if [ -d "$(OUTPUT_DIR)" ] && [ -z "$$(ls -A $(OUTPUT_DIR))" ]; then \
			echo "Removing empty directory $(OUTPUT_DIR)"; \
			$(RMDIR) $(OUTPUT_DIR); \
		fi
	endif
	@echo "Main cleaning complete."

# Help target
.PHONY: help
help:
	@echo "Available targets for $(PROJECT_NAME):"
	@echo "  all           Build the project (default)"
	@echo "  proto         Generate Go code from .proto definitions"
	@echo "  build         Build the project (after generating protos)"
	@echo "  run           Build and run the project"
	@echo "  test          Run Go tests"
	@echo "  fmt           Format Go source code"
	@echo "  install       Build and install the binary to GOBIN or GOPATH/bin"
	@echo "  clean         Remove build artifacts and generated proto files"
	@echo "  clean-proto   Remove only generated Go protobuf and gRPC files"
	@echo ""
	@echo "Version Information:"
	@echo "  PLATFORM:     $(PLATFORM)"
	@echo "  VERSION:      $(VERSION)"
	@echo "  BUILD_TIME:   $(BUILD_TIME)"
	@echo "  GIT_COMMIT:   $(GIT_COMMIT)"
	@echo "  OUTPUT:       $(call fix_path,$(OUTPUT_BINARY))"
	@echo "  PROTO SRC:    $(PROTO_SRC_DIR)"
	@echo "  GENERATED PB: $(GENERATED_PB_DIR)"