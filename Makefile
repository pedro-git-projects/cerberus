SRC_DIR := src
BIN_DIR := bin
BINARY_NAME := $(BIN_DIR)/flow-sentry
MAIN_FILE := main.go

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
