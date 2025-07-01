# Define the Go application name
APP_NAME = distributed-mail-system

# Define the proto file path
PROTO_FILE = proto/mail.proto
PROTO_DIR = proto

# Define the output directory for generated Go proto files
PROTO_GO_OUT = ./proto

# Default target: builds and runs the application
.PHONY: all
all: build run

# Target to generate Go code from proto files
.PHONY: proto
proto:
	@echo "Generating Go code from $(PROTO_FILE)..."
	protoc --go_out=$(PROTO_GO_OUT) --go_opt=paths=source_relative \
	       --go-grpc_out=$(PROTO_GO_OUT) --go-grpc_opt=paths=source_relative \
	       $(PROTO_FILE)
	@echo "Go code generation complete."

# Target to build the Go application
.PHONY: build
build: proto
	@echo "Building $(APP_NAME)..."
	go mod tidy
	go build -o $(APP_NAME) main.go
	@echo "Build complete."

# Target to run the Go application
.PHONY: run
run: build
	@echo "Running $(APP_NAME)..."
	./$(APP_NAME)

# Target to run all Go tests
.PHONY: test
test: build # Ensure the code is built before running tests
	@echo "Running Go tests..."
	go test ./...
	@echo "All tests complete."

# Target to clean up generated files and compiled binary
.PHONY: clean
clean:
	@echo "Cleaning up..."
	rm -f $(APP_NAME)
	rm -f $(PROTO_GO_OUT)/*.pb.go
	@echo "Cleanup complete."

# Target to install Go dependencies
.PHONY: deps
deps:
	@echo "Installing Go dependencies..."
	go mod download
	@echo "Dependencies installed."

# Target to install protoc Go plugins
.PHONY: install-protoc-plugins
install-protoc-plugins:
	@echo "Installing protoc Go plugins..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Protoc Go plugins installed. Ensure GOPATH/bin is in your PATH."
