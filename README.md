# Distributed Mail System in Go

This project implements a simplified distributed mail system using Go and gRPC. It demonstrates inter-service communication, domain-based routing, and graceful shutdown mechanisms.

## Table of Contents
- [Features](#features)
- [Project Structure](#project-structure)
- [Prerequisites](#prerequisites)
- [Setup and Installation](#setup-and-installation)
- [Configuration](#configuration)
- [How to Run](#how-to-run)
- [How to Run Tests](#how-to-run-tests)
- [Graceful Shutdown](#graceful-shutdown)

## Features
- **Nameserver:** Acts as a directory service, mapping email addresses (e.g., `user@domain.com`) to the network address of their responsible Mailbox server. It enforces domain responsibility, rejecting registrations for domains it doesn't manage.
- **Mailbox:** Stores mail messages for users within a specific domain. It can receive mail from the Transfer Server and allow clients to retrieve their mail. Each Mailbox instance is responsible for a particular domain.
- **Transfer Server:** The central component for sending mail. Clients send mail to the Transfer Server, which then queries the Nameserver to find the recipient's Mailbox and forwards the message. Includes retry logic with exponential backoff for mail delivery to Mailboxes.
- **Client:** A simple command-line client to simulate sending and retrieving emails.
- **gRPC Communication:** All inter-service communication is handled using gRPC with Protocol Buffers for efficient and well-defined messaging.
- **Configurable:** Network addresses and domain responsibilities are loaded from a config.json file.
- **Graceful Shutdown:** All server components (Nameserver, Mailbox, Transfer Server) implement graceful shutdown, allowing ongoing operations to complete before the server fully stops, preventing data loss.

## Project Structure
```
GoDissys/
├── proto/
│   └── mail.proto          # Protocol Buffer definitions for gRPC services and messages
│   └── mail.pb.go          # Generated Go code from mail.proto
│   └── mail_grpc.pb.go     # Generated Go gRPC code from mail.proto
├── common/
│   └── common.go           # Configuration loading and common structs
├── nameserver/
│   ├── nameserver.go       # Nameserver implementation
│   └── nameserver_test.go  # Tests for Nameserver
├── mailbox/
│   ├── mailbox.go          # Mailbox server implementation
│   └── mailbox_test.go     # Tests for Mailbox
├── transferserver/
│   ├── transferserver.go   # Transfer Server implementation
│   └── transferserver_test.go # Tests for Transfer Server
├── client/
│   └── client.go           # Client implementation
├── config.json             # Configuration file for service addresses and domains
├── main.go                 # Main application entry point, orchestrates services
└── go.mod                  # Go module definition
└── Makefile                # Automation for building, running, and testing
```

## Prerequisites
Before you begin, ensure you have the following installed:
- **Go (1.18 or higher recommended):** [Download and Install Go](https://golang.org/doc/install)
- **Protocol Buffers Compiler (`protoc`):** 
  Follow the instructions on the [Protocol Buffers GitHub page](https://www.google.com/search?q=https://github.com/protocolbuffers/protobuf%23protocol-compiler-installation).
- **Go gRPC Plugins:** 
  ```
  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
  ```
  Make sure `$GOPATH/bin` is in your system's `PATH` environment variable. You can verify this by running `protoc-gen-go --version`.

## Setup and Installation
1. **Clone the repository (or create the project structure manually):**
    ```
    git clone <your-repo-url>
    cd GoDissys
    ```
    (If you created files manually, ensure you are in the `GoDissys` directory.)

2. **Initialize Go Module:**
    ```
    go mod init GoDissys
    go mod tidy
    ```

3. **Generate Go code from Protocol Buffers:**
    This step compiles your `mail.proto` file into Go source code that gRPC uses.
    ```  
    make proto
    ```
    Alternatively, you can run the `protoc` command directly:
    ```
    protoc --go_out=./proto --go_opt=paths=source_relative \
         --go-grpc_out=./proto --go-grpc_opt=paths=source_relative \
         proto/mail.proto
    ```

## Configuration
The system's network addresses and domain responsibilities are defined in `config.json`.
`config.json` example:
```
{
  "NameserverAddr": "localhost:50051",
  "TransferServerAddr": "localhost:50053",
  "Mailboxes": {
    "earth.com": {
      "Domain": "earth",
      "Addr": "localhost:50054"
    },
    "saturn.com": {
      "Domain": "saturn",
      "Addr": "localhost:50055"
    }
  },
  "NameserverManagedDomains": [
    "earth.com",
    "saturn.com"
  ]
}
```
- `NameserverAddr`: The address where the Nameserver will listen.
- `TransferServerAddr`: The address where the Transfer Server will listen.
- `Mailboxes`: A map defining each Mailbox instance. The key is the full domain name (e.g., `earth.com`), and the value contains the `Domain` alias (for logging) and the `Addr` where that Mailbox will listen.
- `NameserverManagedDomains`: A list of domains that the Nameserver instance is authorized to manage (i.e., accept registrations for).

## How to Run
To build and run the entire distributed mail system:
```
make run
```
This command will:
1. Generate Go code from `mail.proto` (if not already up-to-date).
2. Build the `main.go` application.
3. Execute the compiled application.
You will see logs from the Nameserver, Mailbox instances, Transfer Server, and Client demonstrating the mail flow and service interactions.

## How to Run Tests
To run all unit and integration tests for the project:
```
make test
```
This command will:
1. Build the project.
2. Execute all `_test.go` files in the project, ensuring each component functions correctly.

## Graceful Shutdown
All server components are configured for graceful shutdown. When you press `Ctrl+C` in the terminal where `make run` is executing:
1. Each server will receive an OS interrupt signal (`SIGINT` or `SIGTERM`).
2. They will log that they received the shutdown signal.
3. `grpc.Server.GracefulStop()` will be called, allowing any in-flight gRPC requests to complete within a timeout period.
4. Once all active RPCs are finished (or the timeout is reached), the server will stop listening and its goroutine will exit.
5. The `main.go` function uses a `sync.WaitGroup` to wait for all server goroutines to signal their completion, ensuring the entire application exits cleanly.
