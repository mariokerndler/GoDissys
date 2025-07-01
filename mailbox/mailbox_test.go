package mailbox

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

// TestMailbox_ReceiveAndGetMail tests the ReceiveMail and GetMail functionality.
func TestMailbox_ReceiveAndGetMail(t *testing.T) {
	// Start a test Mailbox server
	lis, err := net.Listen("tcp", "localhost:0") // Use port 0 for a random available port
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	s := grpc.NewServer()
	mailboxService := NewServer()
	proto.RegisterMailboxServer(s, mailboxService)

	go func() {
		if err := s.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			t.Errorf("Mailbox failed to serve: %v", err)
		}
	}()
	defer s.Stop() // Stop the gRPC server when the test finishes

	// Connect to the test Mailbox
	connCtx, connCancel := context.WithTimeout(context.Background(), time.Second)
	defer connCancel()
	conn, err := grpc.DialContext(connCtx, lis.Addr().String(), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		t.Fatalf("Could not connect to Mailbox: %v", err)
	}
	defer conn.Close()
	client := proto.NewMailboxClient(conn)

	testRecipient := "testuser"

	// Test Case 1: Receive a single mail
	t.Run("ReceiveSingleMail", func(t *testing.T) {
		msg := &proto.MailMessage{
			Sender:    "sender1",
			Recipient: testRecipient,
			Subject:   "Test Subject 1",
			Body:      "Test Body 1",
			Timestamp: time.Now().Unix(),
		}
		req := &proto.ReceiveMailRequest{Message: msg}
		resp, err := client.ReceiveMail(context.Background(), req)
		if err != nil {
			t.Fatalf("ReceiveMail failed: %v", err)
		}
		if !resp.GetSuccess() {
			t.Errorf("ReceiveMail expected success, got false. Message: %s", resp.GetMessage())
		}
	})

	// Test Case 2: Receive another mail
	t.Run("ReceiveAnotherMail", func(t *testing.T) {
		msg := &proto.MailMessage{
			Sender:    "sender2",
			Recipient: testRecipient,
			Subject:   "Test Subject 2",
			Body:      "Test Body 2",
			Timestamp: time.Now().Unix(),
		}
		req := &proto.ReceiveMailRequest{Message: msg}
		resp, err := client.ReceiveMail(context.Background(), req)
		if err != nil {
			t.Fatalf("ReceiveMail failed: %v", err)
		}
		if !resp.GetSuccess() {
			t.Errorf("ReceiveMail expected success, got false. Message: %s", resp.GetMessage())
		}
	})

	// Test Case 3: Get mail for the recipient (should retrieve both)
	t.Run("GetMailForRecipient", func(t *testing.T) {
		req := &proto.GetMailRequest{Username: testRecipient}
		resp, err := client.GetMail(context.Background(), req)
		if err != nil {
			t.Fatalf("GetMail failed: %v", err)
		}
		messages := resp.GetMessages()
		if len(messages) != 2 {
			t.Errorf("Expected 2 messages, got %d", len(messages))
		}
		if messages[0].GetSubject() != "Test Subject 1" || messages[1].GetSubject() != "Test Subject 2" {
			t.Errorf("Messages retrieved in unexpected order or content")
		}
	})

	// Test Case 4: Get mail again (should be empty now)
	t.Run("GetMailAgainEmpty", func(t *testing.T) {
		req := &proto.GetMailRequest{Username: testRecipient}
		resp, err := client.GetMail(context.Background(), req)
		if err != nil {
			t.Fatalf("GetMail failed: %v", err)
		}
		messages := resp.GetMessages()
		if len(messages) != 0 {
			t.Errorf("Expected 0 messages after clearing inbox, got %d", len(messages))
		}
	})

	// Test Case 5: Receive mail with empty recipient
	t.Run("ReceiveMailEmptyRecipient", func(t *testing.T) {
		msg := &proto.MailMessage{
			Sender:    "sender",
			Recipient: "", // Empty recipient
			Subject:   "Invalid",
			Body:      "Invalid",
			Timestamp: time.Now().Unix(),
		}
		req := &proto.ReceiveMailRequest{Message: msg}
		_, err := client.ReceiveMail(context.Background(), req)
		if s, ok := status.FromError(err); !ok || s.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error for empty recipient, got %v", err)
		}
	})

	// Test Case 6: Get mail with empty username
	t.Run("GetMailEmptyUsername", func(t *testing.T) {
		req := &proto.GetMailRequest{Username: ""} // Empty username
		_, err := client.GetMail(context.Background(), req)
		if s, ok := status.FromError(err); !ok || s.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error for empty username, got %v", err)
		}
	})
}
