package nameserver

import (
	"GoDissys/proto/proto"
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestNameserver_RegisterAndLookup tests the RegisterMailbox and LookupMailbox functionality.
func TestNameserver_RegisterAndLookup(t *testing.T) {
	// Start a test Nameserver
	lis, err := net.Listen("tcp", "localhost:0") // Use port 0 for a random available port
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	s := grpc.NewServer()
	nameserverService := NewServer()
	proto.RegisterNameserverServer(s, nameserverService)

	go func() {
		if err := s.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			t.Errorf("Nameserver failed to serve: %v", err)
		}
	}()
	defer s.Stop() // Stop the gRPC server when the test finishes

	// Connect to the test Nameserver
	connCtx, connCancel := context.WithTimeout(context.Background(), time.Second)
	defer connCancel()
	conn, err := grpc.DialContext(connCtx, lis.Addr().String(), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		t.Fatalf("Could not connect to Nameserver: %v", err)
	}
	defer conn.Close()
	client := proto.NewNameserverClient(conn)

	// Test Case 1: Register a new mailbox
	t.Run("RegisterNewMailbox", func(t *testing.T) {
		req := &proto.RegisterMailboxRequest{
			Username:       "testuser1",
			MailboxAddress: "localhost:12345",
		}
		resp, err := client.RegisterMailbox(context.Background(), req)
		if err != nil {
			t.Fatalf("RegisterMailbox failed: %v", err)
		}
		if !resp.GetSuccess() {
			t.Errorf("RegisterMailbox expected success, got false. Message: %s", resp.GetMessage())
		}
		if resp.GetMessage() != "Mailbox registered successfully" {
			t.Errorf("RegisterMailbox unexpected message: %s", resp.GetMessage())
		}
	})

	// Test Case 2: Lookup the registered mailbox
	t.Run("LookupExistingMailbox", func(t *testing.T) {
		req := &proto.LookupMailboxRequest{Username: "testuser1"}
		resp, err := client.LookupMailbox(context.Background(), req)
		if err != nil {
			t.Fatalf("LookupMailbox failed: %v", err)
		}
		if !resp.GetFound() {
			t.Errorf("LookupMailbox expected found, got false")
		}
		if resp.GetMailboxAddress() != "localhost:12345" {
			t.Errorf("LookupMailbox expected address 'localhost:12345', got '%s'", resp.GetMailboxAddress())
		}
	})

	// Test Case 3: Register an existing mailbox (should update)
	t.Run("RegisterExistingMailbox", func(t *testing.T) {
		req := &proto.RegisterMailboxRequest{
			Username:       "testuser1",
			MailboxAddress: "localhost:54321", // New address
		}
		resp, err := client.RegisterMailbox(context.Background(), req)
		if err != nil {
			t.Fatalf("RegisterMailbox failed: %v", err)
		}
		if !resp.GetSuccess() {
			t.Errorf("RegisterMailbox expected success, got false. Message: %s", resp.GetMessage())
		}
	})

	// Test Case 4: Lookup the updated mailbox
	t.Run("LookupUpdatedMailbox", func(t *testing.T) {
		req := &proto.LookupMailboxRequest{Username: "testuser1"}
		resp, err := client.LookupMailbox(context.Background(), req)
		if err != nil {
			t.Fatalf("LookupMailbox failed: %v", err)
		}
		if !resp.GetFound() {
			t.Errorf("LookupMailbox expected found, got false")
		}
		if resp.GetMailboxAddress() != "localhost:54321" {
			t.Errorf("LookupMailbox expected updated address 'localhost:54321', got '%s'", resp.GetMailboxAddress())
		}
	})

	// Test Case 5: Lookup a non-existent mailbox
	t.Run("LookupNonExistentMailbox", func(t *testing.T) {
		req := &proto.LookupMailboxRequest{Username: "nonexistentuser"}
		resp, err := client.LookupMailbox(context.Background(), req)
		if err != nil {
			t.Fatalf("LookupMailbox failed: %v", err)
		}
		if resp.GetFound() {
			t.Errorf("LookupMailbox expected not found, got true")
		}
		if resp.GetMailboxAddress() != "" {
			t.Errorf("LookupMailbox expected empty address, got '%s'", resp.GetMailboxAddress())
		}
	})

	// Test Case 6: Register with empty username
	t.Run("RegisterEmptyUsername", func(t *testing.T) {
		req := &proto.RegisterMailboxRequest{
			Username:       "",
			MailboxAddress: "localhost:12345",
		}
		_, err := client.RegisterMailbox(context.Background(), req)
		if s, ok := status.FromError(err); !ok || s.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error, got %v", err)
		}
	})

	// Test Case 7: Lookup with empty username
	t.Run("LookupEmptyUsername", func(t *testing.T) {
		req := &proto.LookupMailboxRequest{Username: ""}
		_, err := client.LookupMailbox(context.Background(), req)
		if s, ok := status.FromError(err); !ok || s.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error, got %v", err)
		}
	})
}
