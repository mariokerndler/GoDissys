package mailbox

import (
	"GoDissys/proto/proto"
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// server is used to implement proto.MailboxServer.
type server struct {
	proto.UnimplementedMailboxServer
	// userInboxes maps full email address to a slice of MailMessage
	userInboxes map[string][]*proto.MailMessage
	mu          sync.RWMutex // Mutex to protect the userInboxes map
	Domain      string
}

// NewServer creates a new Mailbox instance, responsible for the given domain.
func NewServer(domain string) *server {
	return &server{
		userInboxes: make(map[string][]*proto.MailMessage),
		Domain:      domain,
	}
}

// ReceiveMail implements proto.MailboxServer.
// It receives a mail message from the TransferServer and stores it.
func (s *server) ReceiveMail(ctx context.Context, req *proto.ReceiveMailRequest) (*proto.ReceiveMailResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	msg := req.GetMessage()
	if msg == nil {
		return nil, status.Errorf(codes.InvalidArgument, "mail message cannot be empty")
	}
	if msg.RecipientEmail == "" {
		return nil, status.Errorf(codes.InvalidArgument, "recipient email cannot be empty")
	}

	s.userInboxes[msg.RecipientEmail] = append(s.userInboxes[msg.RecipientEmail], msg)
	log.Printf("Mailbox '%s' for '%s': Received new mail from '%s' (Subject: %s)",
		s.Domain, msg.RecipientEmail, msg.SenderEmail, msg.Subject) // Used s.Domain in log

	return &proto.ReceiveMailResponse{Success: true, Message: "Mail received successfully"}, nil
}

// GetMail implements proto.MailboxServer.
// It retrieves all messages for a given email address and then clears their inbox.
func (s *server) GetMail(ctx context.Context, req *proto.GetMailRequest) (*proto.GetMailResponse, error) {
	s.mu.Lock() // Use Lock because we modify the map (clearing inbox)
	defer s.mu.Unlock()

	emailAddress := req.GetEmailAddress()
	if emailAddress == "" {
		return nil, status.Errorf(codes.InvalidArgument, "email address cannot be empty")
	}

	messages, found := s.userInboxes[emailAddress]
	if !found || len(messages) == 0 {
		log.Printf("Mailbox '%s' for '%s': No new mail to retrieve", s.Domain, emailAddress)
		return &proto.GetMailResponse{Messages: []*proto.MailMessage{}}, nil
	}

	// Create a copy of messages to return
	msgsToReturn := make([]*proto.MailMessage, len(messages))
	copy(msgsToReturn, messages)

	// Clear the inbox for the user after retrieval
	s.userInboxes[emailAddress] = []*proto.MailMessage{} // Reset to empty slice
	log.Printf("Mailbox '%s' for '%s': Retrieved %d messages and cleared inbox", s.Domain, emailAddress, len(msgsToReturn))

	return &proto.GetMailResponse{Messages: msgsToReturn}, nil
}

// StartMailbox starts the gRPC server for the Mailbox on a specific address.
// It also sets up graceful shutdown.
func StartMailbox(domain, mailboxAddr string) {
	lis, err := net.Listen("tcp", mailboxAddr)
	if err != nil {
		log.Printf("Mailbox '%s' failed to listen on %s: %v", domain, mailboxAddr, err)
		return // Return instead of Fatalf, allow main to handle
	}

	s := grpc.NewServer()
	mailboxService := NewServer(domain) // Pass domain to NewServer
	proto.RegisterMailboxServer(s, mailboxService)
	log.Printf("Mailbox '%s' listening on %s", domain, mailboxAddr)

	// Goroutine to serve gRPC requests
	go func() {
		if err := s.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			log.Printf("Mailbox '%s' failed to serve: %v", domain, err)
		}
	}()

	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit // Block until a signal is received
	log.Printf("Mailbox '%s' received shutdown signal. Shutting down gracefully...", domain)
	s.GracefulStop() // Gracefully stop the gRPC server
	log.Printf("Mailbox '%s' server stopped.", domain)
}

// RegisterMailboxWithNameserver connects to the Nameserver and registers this mailbox for a specific email.
func RegisterMailboxWithNameserver(nameserverAddr, emailAddress, mailboxAddr string) {
	ctxDial, cancelDial := context.WithTimeout(context.Background(), time.Second*5)
	defer cancelDial()

	conn, err := grpc.DialContext(ctxDial, nameserverAddr, grpc.WithInsecure()) // Use nameserverAddr
	if err != nil {
		log.Fatalf("Mailbox: Could not connect to Nameserver at %s: %v", nameserverAddr, err)
	}
	defer conn.Close()

	client := proto.NewNameserverClient(conn)

	ctxReq, cancelReq := context.WithTimeout(context.Background(), time.Second*5)
	defer cancelReq()

	req := &proto.RegisterMailboxRequest{
		EmailAddress:   emailAddress,
		MailboxAddress: mailboxAddr,
	}

	resp, err := client.RegisterMailbox(ctxReq, req)
	if err != nil {
		log.Fatalf("Mailbox: Could not register '%s' with Nameserver: %v", emailAddress, err)
	}
	if resp.GetSuccess() {
		log.Printf("Mailbox: Successfully registered '%s' with Nameserver: %s", emailAddress, resp.GetMessage())
	} else {
		log.Fatalf("Mailbox: Failed to register '%s' with Nameserver: %s", emailAddress, resp.GetMessage())
	}
}
