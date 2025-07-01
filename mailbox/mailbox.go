package mailbox

import (
	"GoDissys/common"
	"GoDissys/proto/proto"
	"context"
	"log"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// server is used to implement proto.MailboxServer
type server struct {
	proto.UnimplementedMailboxServer

	// userInbox maps username to a slice of MailMessage
	userInboxes map[string][]*proto.MailMessage
	mu          sync.RWMutex // Mutex to protext the userInboxes map
}

// NewServer creates a new Mailbox instance
func NewServer() *server {
	return &server{
		userInboxes: make(map[string][]*proto.MailMessage),
	}
}

// StartMailbox starts the gRPC server for the Mailbox
func StartMailbox(domain, port string) {
	addr := "localhost:" + port
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Mailbox failed to listen on %s: %v", addr, err)
	}
	s := grpc.NewServer()
	mailboxService := NewServer()
	proto.RegisterMailboxServer(s, mailboxService)
	log.Printf("Mailbox '%s' listening on %s", domain, addr)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Mailbox failed to serve on %s: %v", addr, err)
	}
}

// RegisterMailboxWithNameserver connects to the Nameserver and registers the mailbox
func RegisterMailboxWithNameserver(emailAddress, mailboxAddr string) {
	ctxDial, cancelDial := context.WithTimeout(context.Background(), time.Second*5)
	defer cancelDial()

	conn, err := grpc.DialContext(ctxDial, common.NameserverAddr, grpc.WithInsecure()) // Insecure for practice, use TLS in production
	if err != nil {
		log.Fatalf("Mailbox: Could not connect to Nameserver at %s: %v", common.NameserverAddr, err)
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

// ReceiveMail implements proto.MailboxServer
// It receives a mail message from the TransferServer and stores it
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
	log.Printf("Mailbox for '%s': Received new mail from '%s' (Subject: %s)",
		msg.RecipientEmail, msg.SenderEmail, msg.Subject)

	return &proto.ReceiveMailResponse{Success: true, Message: "Mail received successfully"}, nil
}

// GetMail implements proto.MailboxServer
// It retrieves all messages for a given user and then clears their inbox
func (s *server) GetMail(ctx context.Context, req *proto.GetMailRequest) (*proto.GetMailResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	emailAddress := req.GetEmailAddress()
	if emailAddress == "" {
		return nil, status.Errorf(codes.InvalidArgument, "email address cannot be empty")
	}

	messages, found := s.userInboxes[emailAddress]
	if !found || len(messages) == 0 {
		log.Printf("Mailbox for '%s': No new mail to retrieve", emailAddress)
		return &proto.GetMailResponse{Messages: []*proto.MailMessage{}}, nil
	}

	// Create a copy of messages to return
	msgsToReturn := make([]*proto.MailMessage, len(messages))
	copy(msgsToReturn, messages)

	// Clear the inbox for the user after retrieval
	s.userInboxes[emailAddress] = []*proto.MailMessage{} // Reset to empty slice
	log.Printf("Mailbox for '%s': Retrieved %d messages and cleared inbox", emailAddress, len(msgsToReturn))

	return &proto.GetMailResponse{Messages: msgsToReturn}, nil
}
