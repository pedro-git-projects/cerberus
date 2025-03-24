SRC_DIR := src
BIN_DIR := bin
BINARY_NAME := $(BIN_DIR)/flow-sentry
MAIN_FILE := main.go

TESTSUITES ?= all
DEPLOYWORKFLOWS ?= none

.PHONY: all
all: build

.PHONY: build
build:
	@echo "🔨 Building $(BINARY_NAME)..."
	@mkdir -p $(BIN_DIR)
	cd $(SRC_DIR) && go build -o ../$(BINARY_NAME) $(MAIN_FILE)

.PHONY: run
run: build
	@echo "🚀 Running $(BINARY_NAME)..."
	./$(BINARY_NAME)

# 🔧 Default: run all test suites, no deployment
.PHONY: run-default
run-default: build
	@echo "🚀 Running default (testsuites=all, deployWorkflows=none)..."
	./$(BINARY_NAME) -testsuites=all -deployWorkflows=none

# 🔧 Run everything: all test suites, all workflows
.PHONY: run-all
run-all: build
	@echo "🚀 Running full test + full deployment..."
	./$(BINARY_NAME) -testsuites=all -deployWorkflows=all

# 🔧 Run a specific suite and deploy from that suite
.PHONY: run-suite
run-suite: build
	@echo "🚀 Running specific suite and deploying suite workflows..."
	./$(BINARY_NAME) -testsuites=invoice-process -deployWorkflows=suite

# 🔧 Run with custom env-supplied variables
.PHONY: run-custom
run-custom: build
	@echo "🚀 Running with custom args: TESTSUITES=$(TESTSUITES), DEPLOYWORKFLOWS=$(DEPLOYWORKFLOWS)"
	./$(BINARY_NAME) -testsuites=$(TESTSUITES) -deployWorkflows=$(DEPLOYWORKFLOWS)

.PHONY: test
test:
	@echo "🧪 Running tests..."
	cd $(SRC_DIR) && go test ./...

.PHONY: fmt
fmt:
	@echo "🎨 Formatting code..."
	cd $(SRC_DIR) && go fmt ./...

.PHONY: clean
clean:
	@echo "🧹 Cleaning up..."
	go clean
	rm -rf $(BIN_DIR)
