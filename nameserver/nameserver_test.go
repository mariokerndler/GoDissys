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

// TestNameserver_RegisterAndLookup tests the RegisterMailbox and LookupMailbox functionality with email addresses.
func TestNameserver_RegisterAndLookup(t *testing.T) {
	// Start a test Nameserver, responsible for "earth.com" and "saturn.com"
	testDomains := []string{"earth.com", "saturn.com"}
	lis, err := net.Listen("tcp", "localhost:0") // Use port 0 for a random available port
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	nameserverAddr := lis.Addr().String() // Get the actual address
	s := grpc.NewServer()
	nameserverService := NewServer(testDomains) // Pass responsible domains
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
	conn, err := grpc.DialContext(connCtx, nameserverAddr, grpc.WithInsecure(), grpc.WithBlock()) // Use nameserverAddr
	if err != nil {
		t.Fatalf("Could not connect to Nameserver: %v", err)
	}
	defer conn.Close()
	client := proto.NewNameserverClient(conn)

	// Test Case 1: Register a new mailbox for an email address within a managed domain
	t.Run("RegisterNewMailboxManagedDomain", func(t *testing.T) {
		req := &proto.RegisterMailboxRequest{
			EmailAddress:   "alice@earth.com",
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
		req := &proto.LookupMailboxRequest{EmailAddress: "alice@earth.com"}
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

	// Test Case 3: Register another mailbox for a different managed domain
	t.Run("RegisterAnotherManagedDomainMailbox", func(t *testing.T) {
		req := &proto.RegisterMailboxRequest{
			EmailAddress:   "bob@saturn.com",
			MailboxAddress: "localhost:67890",
		}
		resp, err := client.RegisterMailbox(context.Background(), req)
		if err != nil {
			t.Fatalf("RegisterMailbox failed: %v", err)
		}
		if !resp.GetSuccess() {
			t.Errorf("RegisterMailbox expected success, got false. Message: %s", resp.GetMessage())
		}
	})

	// Test Case 4: Lookup the newly registered domain mailbox
	t.Run("LookupAnotherManagedDomainMailbox", func(t *testing.T) {
		req := &proto.LookupMailboxRequest{EmailAddress: "bob@saturn.com"}
		resp, err := client.LookupMailbox(context.Background(), req)
		if err != nil {
			t.Fatalf("LookupMailbox failed: %v", err)
		}
		if !resp.GetFound() {
			t.Errorf("LookupMailbox expected found, got false")
		}
		if resp.GetMailboxAddress() != "localhost:67890" {
			t.Errorf("LookupMailbox expected address 'localhost:67890', got '%s'", resp.GetMailboxAddress())
		}
	})

	// Test Case 5: Register an existing email address (should update)
	t.Run("RegisterExistingEmailAddress", func(t *testing.T) {
		req := &proto.RegisterMailboxRequest{
			EmailAddress:   "alice@earth.com",
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

	// Test Case 6: Lookup the updated mailbox
	t.Run("LookupUpdatedMailbox", func(t *testing.T) {
		req := &proto.LookupMailboxRequest{EmailAddress: "alice@earth.com"}
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

	// Test Case 7: Register an email address for an unmanaged domain (should be rejected)
	t.Run("RegisterUnmanagedDomain", func(t *testing.T) {
		req := &proto.RegisterMailboxRequest{
			EmailAddress:   "diana@mars.com", // This domain is not in testDomains
			MailboxAddress: "localhost:99999",
		}
		resp, err := client.RegisterMailbox(context.Background(), req)
		if err != nil {
			t.Fatalf("RegisterMailbox failed: %v", err)
		}
		if resp.GetSuccess() {
			t.Errorf("RegisterMailbox expected failure for unmanaged domain, got success")
		}
		expectedMsg := "Domain 'mars.com' is not managed by this Nameserver."
		if resp.GetMessage() != expectedMsg {
			t.Errorf("Expected message '%s', got '%s'", expectedMsg, resp.GetMessage())
		}
	})

	// Test Case 8: Lookup a non-existent email address
	t.Run("LookupNonExistentEmailAddress", func(t *testing.T) {
		req := &proto.LookupMailboxRequest{EmailAddress: "nonexistent@example.com"}
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

	// Test Case 9: Register with empty email address
	t.Run("RegisterEmptyEmailAddress", func(t *testing.T) {
		req := &proto.RegisterMailboxRequest{
			EmailAddress:   "",
			MailboxAddress: "localhost:12345",
		}
		_, err := client.RegisterMailbox(context.Background(), req)
		if s, ok := status.FromError(err); !ok || s.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error, got %v", err)
		}
	})

	// Test Case 10: Lookup with empty email address
	t.Run("LookupEmptyEmailAddress", func(t *testing.T) {
		req := &proto.LookupMailboxRequest{EmailAddress: ""}
		_, err := client.LookupMailbox(context.Background(), req)
		if s, ok := status.FromError(err); !ok || s.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error, got %v", err)
		}
	})
}
