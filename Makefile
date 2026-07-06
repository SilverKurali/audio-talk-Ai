.PHONY: build install test clean run

APP_NAME := audio-talk-ai
CMD_DIR := ./cmd/audio-talk-ai
BUILD_DIR := ./build

# Build for current platform
build:
	go build -o $(BUILD_DIR)/$(APP_NAME) $(CMD_DIR)

# Install to ~/.local/bin
install: build
	$(BUILD_DIR)/$(APP_NAME) --install
	chmod +x bin/daudio-talk-ai
	cp bin/daudio-talk-ai ~/.local/bin/daudio-talk-ai

# Run (current platform)
run:
	go run $(CMD_DIR)

# Test
test:
	go test ./... -v

# Clean
clean:
	rm -rf $(BUILD_DIR)

# Install dependencies
deps:
	go mod tidy
	go mod download
