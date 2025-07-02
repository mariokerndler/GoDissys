package transferserver

import (
	"GoDissys/proto/proto"
	"context"
	"fmt"
	"net"
	"strings" // Import for strings.Contains
	"sync"
	"sync/atomic" // For atomic counter in mock
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MockNameserverClient is a mock implementation of proto.NameserverClient for testing.
type MockNameserverClient struct {
	mu        sync.RWMutex
	mailboxes map[string]string // email_address -> mailbox address
}

func NewMockNameserverClient() *MockNameserverClient {
	return &MockNameserverClient{
		mailboxes: make(map[string]string),
	}
}

func (m *MockNameserverClient) RegisterMailbox(ctx context.Context, in *proto.RegisterMailboxRequest, opts ...grpc.CallOption) (*proto.RegisterMailboxResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mailboxes[in.GetEmailAddress()] = in.GetMailboxAddress()
	return &proto.RegisterMailboxResponse{Success: true, Message: "Mock registered"}, nil
}

func (m *MockNameserverClient) LookupMailbox(ctx context.Context, in *proto.LookupMailboxRequest, opts ...grpc.CallOption) (*proto.LookupMailboxResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	addr, found := m.mailboxes[in.GetEmailAddress()]
	return &proto.LookupMailboxResponse{Found: found, MailboxAddress: addr}, nil
}

// MockMailboxServer is a mock implementation of proto.MailboxServer for testing.
type MockMailboxServer struct {
	proto.UnimplementedMailboxServer
	receivedMessages []*proto.MailMessage
	mu               sync.Mutex
	// failCount is used to simulate transient failures.
	// The server will return an error for the first `failCount` ReceiveMail calls.
	failCount int32
	callCount int32
}

func NewMockMailboxServer(failBeforeSuccess int32) *MockMailboxServer {
	return &MockMailboxServer{
		receivedMessages: make([]*proto.MailMessage, 0),
		failCount:        failBeforeSuccess,
	}
}

func (m *MockMailboxServer) ReceiveMail(ctx context.Context, req *proto.ReceiveMailRequest) (*proto.ReceiveMailResponse, error) {
	atomic.AddInt32(&m.callCount, 1)
	if atomic.LoadInt32(&m.callCount) <= m.failCount {
		return nil, status.Errorf(codes.Unavailable, "mock mailbox unavailable (simulated transient error)")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.receivedMessages = append(m.receivedMessages, req.GetMessage())
	return &proto.ReceiveMailResponse{Success: true, Message: "Mock mail received"}, nil
}

func (m *MockMailboxServer) GetMail(ctx context.Context, req *proto.GetMailRequest) (*proto.GetMailResponse, error) {
	// Not directly used by TransferServer, but implemented for completeness if needed later
	return &proto.GetMailResponse{Messages: []*proto.MailMessage{}}, nil
}

// TestTransferServer_SendMail tests the SendMail functionality of the TransferServer.
func TestTransferServer_SendMail(t *testing.T) {
	// 1. Setup Mock Nameserver Client
	mockNameserver := NewMockNameserverClient()

	// 2. Start TransferServer with Mock Nameserver Client
	transferLis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to listen for transfer server: %v", err)
	}
	transferServerAddr := transferLis.Addr().String()
	transferSrv := grpc.NewServer()
	transferServerService := NewServer(mockNameserver) // Inject the mock nameserver client
	proto.RegisterTransferServerServer(transferSrv, transferServerService)
	go func() {
		if err := transferSrv.Serve(transferLis); err != nil && err != grpc.ErrServerStopped {
			t.Errorf("TransferServer failed to serve: %v", err)
		}
	}()
	defer transferSrv.Stop()

	// Connect to the test TransferServer
	connCtx, connCancel := context.WithTimeout(context.Background(), time.Second)
	defer connCancel()
	conn, err := grpc.DialContext(connCtx, transferServerAddr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		t.Fatalf("Could not connect to TransferServer: %v", err)
	}
	defer conn.Close()
	client := proto.NewTransferServerClient(conn)

	// Test Case 1: Successfully send mail to a registered recipient (no failures)
	t.Run("SendMailSuccessNoFailure", func(t *testing.T) {
		// Start Mock Mailbox Server that always succeeds
		mockMailbox := NewMockMailboxServer(0) // 0 failures
		mailboxLis, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			t.Fatalf("Failed to listen for mock mailbox: %v", err)
		}
		mailboxSrv := grpc.NewServer()
		proto.RegisterMailboxServer(mailboxSrv, mockMailbox)
		go func() {
			if err := mailboxSrv.Serve(mailboxLis); err != nil && err != grpc.ErrServerStopped {
				t.Errorf("Mock Mailbox failed to serve: %v", err)
			}
		}()
		defer mailboxSrv.Stop()
		mailboxAddr := mailboxLis.Addr().String()
		mockNameserver.RegisterMailbox(context.Background(), &proto.RegisterMailboxRequest{
			EmailAddress:   "recipient1@example.com",
			MailboxAddress: mailboxAddr,
		})

		msg := &proto.MailMessage{
			SenderEmail:    "senderA@domain.com",
			RecipientEmail: "recipient1@example.com",
			Subject:        "Hello Recipient1",
			Body:           "This is a test email.",
			Timestamp:      time.Now().Unix(),
		}
		req := &proto.SendMailRequest{Message: msg}
		resp, err := client.SendMail(context.Background(), req)
		if err != nil {
			t.Fatalf("SendMail failed: %v", err)
		}
		if !resp.GetSuccess() {
			t.Errorf("SendMail expected success, got false. Message: %s", resp.GetMessage())
		}

		time.Sleep(time.Millisecond * 100) // Give a moment for async processing
		mockMailbox.mu.Lock()
		defer mockMailbox.mu.Unlock()
		if len(mockMailbox.receivedMessages) != 1 {
			t.Errorf("Expected 1 message in mock mailbox, got %d", len(mockMailbox.receivedMessages))
		}
		if mockMailbox.receivedMessages[0].GetSubject() != "Hello Recipient1" {
			t.Errorf("Received message subject mismatch: got %s", mockMailbox.receivedMessages[0].GetSubject())
		}
		if mockMailbox.callCount != 1 {
			t.Errorf("Expected 1 call to ReceiveMail, got %d", mockMailbox.callCount)
		}
	})

	// Test Case 2: Successfully send mail after transient failures
	t.Run("SendMailSuccessWithRetries", func(t *testing.T) {
		// Start Mock Mailbox Server that fails 2 times, then succeeds
		mockMailbox := NewMockMailboxServer(2) // Fails 2 times
		mailboxLis, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			t.Fatalf("Failed to listen for mock mailbox: %v", err)
		}
		mailboxSrv := grpc.NewServer()
		proto.RegisterMailboxServer(mailboxSrv, mockMailbox)
		go func() {
			if err := mailboxSrv.Serve(mailboxLis); err != nil && err != grpc.ErrServerStopped {
				t.Errorf("Mock Mailbox failed to serve: %v", err)
			}
		}()
		defer mailboxSrv.Stop()
		mailboxAddr := mailboxLis.Addr().String()
		mockNameserver.RegisterMailbox(context.Background(), &proto.RegisterMailboxRequest{
			EmailAddress:   "recipient2@example.com",
			MailboxAddress: mailboxAddr,
		})

		msg := &proto.MailMessage{
			SenderEmail:    "senderB@domain.com",
			RecipientEmail: "recipient2@example.com",
			Subject:        "Hello Recipient2",
			Body:           "This is a test email with retries.",
			Timestamp:      time.Now().Unix(),
		}
		req := &proto.SendMailRequest{Message: msg}
		resp, err := client.SendMail(context.Background(), req)
		if err != nil {
			t.Fatalf("SendMail failed: %v", err)
		}
		if !resp.GetSuccess() {
			t.Errorf("SendMail expected success, got false. Message: %s", resp.GetMessage())
		}

		time.Sleep(time.Millisecond * 100) // Give a moment for async processing
		mockMailbox.mu.Lock()
		defer mockMailbox.mu.Unlock()
		if len(mockMailbox.receivedMessages) != 1 {
			t.Errorf("Expected 1 message in mock mailbox, got %d", len(mockMailbox.receivedMessages))
		}
		if mockMailbox.receivedMessages[0].GetSubject() != "Hello Recipient2" {
			t.Errorf("Received message subject mismatch: got %s", mockMailbox.receivedMessages[0].GetSubject())
		}
		// Expected calls: 2 failures + 1 success = 3 calls
		if mockMailbox.callCount != 3 {
			t.Errorf("Expected 3 calls to ReceiveMail (2 failures + 1 success), got %d", mockMailbox.callCount)
		}
	})

	// Test Case 3: Send mail fails after all retries are exhausted
	t.Run("SendMailFailureAfterRetries", func(t *testing.T) {
		// Start Mock Mailbox Server that always fails (more than maxRetries)
		mockMailbox := NewMockMailboxServer(maxRetries + 1) // Fails more than maxRetries
		mailboxLis, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			t.Fatalf("Failed to listen for mock mailbox: %v", err)
		}
		mailboxSrv := grpc.NewServer()
		proto.RegisterMailboxServer(mailboxSrv, mockMailbox)
		go func() {
			if err := mailboxSrv.Serve(mailboxLis); err != nil && err != grpc.ErrServerStopped {
				t.Errorf("Mock Mailbox failed to serve: %v", err)
			}
		}()
		defer mailboxSrv.Stop()
		mailboxAddr := mailboxLis.Addr().String()
		mockNameserver.RegisterMailbox(context.Background(), &proto.RegisterMailboxRequest{
			EmailAddress:   "recipient3@example.com",
			MailboxAddress: mailboxAddr,
		})

		msg := &proto.MailMessage{
			SenderEmail:    "senderC@domain.com",
			RecipientEmail: "recipient3@example.com",
			Subject:        "Failed Mail",
			Body:           "This email should not be delivered.",
			Timestamp:      time.Now().Unix(),
		}
		req := &proto.SendMailRequest{Message: msg}
		resp, err := client.SendMail(context.Background(), req)
		if err != nil {
			t.Fatalf("SendMail failed: %v", err)
		}
		if resp.GetSuccess() {
			t.Errorf("SendMail expected failure, got success")
		}
		// Check if the message contains the expected parts
		expectedPart1 := fmt.Sprintf("Mail delivery failed after %d retries:", maxRetries)
		expectedPart2 := "mock mailbox unavailable (simulated transient error)"
		if !strings.Contains(resp.GetMessage(), expectedPart1) || !strings.Contains(resp.GetMessage(), expectedPart2) {
			t.Errorf("Unexpected error message.\nExpected to contain: '%s' and '%s'\nActual: '%s'",
				expectedPart1, expectedPart2, resp.GetMessage())
		}

		time.Sleep(time.Millisecond * 100) // Give a moment for async processing
		mockMailbox.mu.Lock()
		defer mockMailbox.mu.Unlock()
		if len(mockMailbox.receivedMessages) != 0 {
			t.Errorf("Expected 0 messages in mock mailbox, got %d", len(mockMailbox.receivedMessages))
		}
		// Expected calls: maxRetries + 1 (initial attempt + retries)
		if mockMailbox.callCount != maxRetries+1 {
			t.Errorf("Expected %d calls to ReceiveMail, got %d", maxRetries+1, mockMailbox.callCount)
		}
	})

	// Test Case 4: Send mail to an unregistered recipient (should fail quickly without retries for delivery)
	t.Run("SendMailUnregisteredRecipient", func(t *testing.T) {
		msg := &proto.MailMessage{
			SenderEmail:    "senderD@domain.com",
			RecipientEmail: "unknownuser@unknown.com",
			Subject:        "To Unknown",
			Body:           "This should fail.",
			Timestamp:      time.Now().Unix(),
		}
		req := &proto.SendMailRequest{Message: msg}
		resp, err := client.SendMail(context.Background(), req)
		if err != nil {
			t.Fatalf("SendMail failed: %v", err)
		}
		if resp.GetSuccess() {
			t.Errorf("SendMail expected failure for unknown user, got success")
		}
		if resp.GetMessage() != "Recipient 'unknownuser@unknown.com' not found" {
			t.Errorf("Expected 'Recipient not found' message, got '%s'", resp.GetMessage())
		}
	})

	// Test Case 5: Send mail with empty recipient email
	t.Run("SendMailEmptyRecipientEmail", func(t *testing.T) {
		msg := &proto.MailMessage{
			SenderEmail:    "senderE@domain.com",
			RecipientEmail: "", // Empty recipient email
			Subject:        "Invalid Mail",
			Body:           "This should cause an error.",
			Timestamp:      time.Now().Unix(),
		}
		req := &proto.SendMailRequest{Message: msg}
		_, err := client.SendMail(context.Background(), req)
		if s, ok := status.FromError(err); !ok || s.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error for empty recipient email, got %v", err)
		}
	})
}
