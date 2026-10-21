# ==============================================================================
# Variables
# ==============================================================================
BINARY_NAME=ttf2bmp
OUTPUT_DIR=bin

# Directory Definitions
# DEMO_OUT_DIR: Where 'make run' saves files. 'make verify' reads from here.
DEMO_OUT_DIR=output
# TEST_OUT_DIR: Where 'make validate' saves isolated test files.
TEST_OUT_DIR=test_output
# TEST_DATA_DIR: Where fonts are downloaded.
TEST_DATA_DIR=test_data

# Tools Source
TOOL_VERIFIER_SRC=./tools/verifier/main.go
TOOL_VALIDATOR_SRC=./tools/validator/main.go

# Local Build Binaries (Host OS)
TOOL_VERIFIER_BIN=$(OUTPUT_DIR)/verifier
TOOL_VALIDATOR_BIN=$(OUTPUT_DIR)/validator

# Versioning
VERSION := $(shell git describe --tags --always --dirty || echo "dev")
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"

# ------------------------------------------------------------------------------
# Test Configuration
# ------------------------------------------------------------------------------
TEST_FONT_NAME ?= Go-Regular
TEST_FONT_URL  ?= https://github.com/golang/image/raw/master/font/gofont/ttfs/Go-Regular.ttf
TEST_CHARS     ?= "Abdfghjlpqty123"
TEST_SIZE      ?= 32

# Derived Paths
TEST_FONT_PATH  = $(TEST_DATA_DIR)/$(TEST_FONT_NAME).ttf

# Go Commands
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOVET=$(GOCMD) vet
GOFMT=$(GOCMD) fmt
GOLINT=golangci-lint

# ==============================================================================
# Main Targets
# ==============================================================================

.PHONY: all build clean test bench deps vet lint format run help verify validate fetch-test-data \
        build-cli build-verifier build-validator build-lib build-tools \
        build-linux build-windows build-mac build-all

# Default target
all: deps check test build

deps:
	@echo "  >  Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# ==============================================================================
# Build Targets
# ==============================================================================

build: build-lib build-cli build-tools

build-cli:
	@echo "  >  Building CLI ($(BINARY_NAME))..."
	@mkdir -p $(OUTPUT_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME) .

build-lib:
	@echo "  >  Verifying Library compilation..."
	$(GOBUILD) ./converter/...

build-tools: build-verifier build-validator

build-verifier:
	@echo "  >  Building Verifier Tool..."
	@mkdir -p $(OUTPUT_DIR)
	$(GOBUILD) -o $(TOOL_VERIFIER_BIN) $(TOOL_VERIFIER_SRC)

build-validator:
	@echo "  >  Building Validator Tool..."
	@mkdir -p $(OUTPUT_DIR)
	$(GOBUILD) -o $(TOOL_VALIDATOR_BIN) $(TOOL_VALIDATOR_SRC)

# ==============================================================================
# Test Data
# ==============================================================================

fetch-test-data:
	@mkdir -p $(TEST_DATA_DIR)
	@if [ ! -f "$(TEST_FONT_PATH)" ]; then \
		echo "  >  Downloading test font ($(TEST_FONT_NAME))..."; \
		curl -s -L -o "$(TEST_FONT_PATH)" "$(TEST_FONT_URL)"; \
	else \
		echo "  >  Using existing font at $(TEST_FONT_PATH)"; \
	fi

# ==============================================================================
# Testing & Code Quality
# ==============================================================================

test: fetch-test-data
	@echo "  >  Running tests..."
	$(GOTEST) -v ./...

check: vet lint

vet:
	@echo "  >  Vetting code..."
	$(GOVET) ./...

lint:
	@echo "  >  Running Linter..."
	$(GOLINT) run ./...

format:
	@echo "  >  Formatting code..."
	$(GOFMT) ./...

# ==============================================================================
# Execution & Pipelines
# ==============================================================================

# 1. RUN (Demo)
# Runs the CLI using DEMO_OUT_DIR for output.
run: build-cli fetch-test-data
	@echo "  >  Running Demo Conversion (Default PNG)..."
	@rm -rf $(DEMO_OUT_DIR)
	@mkdir -p $(DEMO_OUT_DIR)
	@./$(OUTPUT_DIR)/$(BINARY_NAME) \
		-f "$(TEST_FONT_PATH)" \
		-s "$(TEST_SIZE)" \
		-c $(TEST_CHARS) \
		-o $(DEMO_OUT_DIR)
	@echo "  >  Output generated in $(DEMO_OUT_DIR)/"

# 2. VERIFY (Visual Check)
# Reads from DEMO_OUT_DIR (must match 'run').
verify: run build-verifier
	@echo "  >  Running Visual Verification..."
	@rm -rf $(TEST_OUT_DIR)/verifier
	@mkdir -p $(TEST_OUT_DIR)/verifier
	@./$(TOOL_VERIFIER_BIN) \
		-fnt "$(DEMO_OUT_DIR)/$(TEST_FONT_NAME)-$(TEST_SIZE).fnt" \
		-out "$(TEST_OUT_DIR)/verifier"

# 3. VALIDATE (Mathematical Regression)
# Forces BMP and uses isolated TEST_OUT_DIR.
validate: build-cli build-validator fetch-test-data
	@echo "  >  Running Validation Pipeline (Force BMP)..."
	@rm -rf $(TEST_OUT_DIR)/validator
	@mkdir -p $(TEST_OUT_DIR)/validator
	@./$(OUTPUT_DIR)/$(BINARY_NAME) \
		-f "$(TEST_FONT_PATH)" \
		-s "$(TEST_SIZE)" \
		-c $(TEST_CHARS) \
		-t "bmp" \
		-o $(TEST_OUT_DIR)/validator
	@echo "  >  Verifying output math..."
	@./$(TOOL_VALIDATOR_BIN) \
		-ttf "$(TEST_FONT_PATH)" \
		-fnt "$(TEST_OUT_DIR)/validator/$(TEST_FONT_NAME)-$(TEST_SIZE).fnt" \
		-chars $(TEST_CHARS) \
		-size $(TEST_SIZE) \
		-out $(TEST_OUT_DIR)/validator

# ==============================================================================
# Cleanup
# ==============================================================================

clean:
	@echo "  >  Cleaning build artifacts..."
	$(GOCLEAN)
	rm -rf $(OUTPUT_DIR) $(DEMO_OUT_DIR) $(TEST_OUT_DIR) $(TEST_DATA_DIR)

# ==============================================================================
# Cross Compilation
# ==============================================================================

build-linux:
	@echo "  >  Building for Linux (AMD64)..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME)-linux .
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(OUTPUT_DIR)/verifier-linux $(TOOL_VERIFIER_SRC)
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(OUTPUT_DIR)/validator-linux $(TOOL_VALIDATOR_SRC)

build-windows:
	@echo "  >  Building for Windows (AMD64)..."
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME)-windows.exe .
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(OUTPUT_DIR)/verifier-windows.exe $(TOOL_VERIFIER_SRC)
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(OUTPUT_DIR)/validator-windows.exe $(TOOL_VALIDATOR_SRC)

build-mac:
	@echo "  >  Building for MacOS (Intel)..."
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(OUTPUT_DIR)/verifier-darwin-amd64 $(TOOL_VERIFIER_SRC)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(OUTPUT_DIR)/validator-darwin-amd64 $(TOOL_VALIDATOR_SRC)
	@echo "  >  Building for MacOS (Apple Silicon)..."
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME)-darwin-arm64 .
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -o $(OUTPUT_DIR)/verifier-darwin-arm64 $(TOOL_VERIFIER_SRC)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -o $(OUTPUT_DIR)/validator-darwin-arm64 $(TOOL_VALIDATOR_SRC)

build-all: build-linux build-mac build-windows

help:
	@echo "Usage: make [target] [VARIABLES]"
	@echo ""
	@echo "Targets:"
	@echo "  run          - Run Demo conversion (outputs to '$(DEMO_OUT_DIR)/')"
	@echo "  verify       - Visual check of Demo output"
	@echo "  validate     - Math regression test (outputs to '$(TEST_OUT_DIR)/validator')"
	@echo "  clean        - Remove all artifacts"
	@echo "  build-all    - Cross-compile for all OS"