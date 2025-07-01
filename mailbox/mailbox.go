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
func StartMailbox() {
	lis, err := net.Listen("tcp", common.MailboxAddr)
	if err != nil {
		log.Fatalf("Mailbox failed to listen: %v", err)
	}

	s := grpc.NewServer()
	mailboxService := NewServer()
	proto.RegisterMailboxServer(s, mailboxService)
	log.Printf("Mailbox listening on %s", common.MailboxAddr)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Mailbox failed to serve: %v", err)
	}
}

// RegisterMailboxWithNameserver connects to the Nameserver and registers the mailbox
func RegisterMailboxWithNameserver(username string) {
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
		Username:       username,
		MailboxAddress: common.MailboxAddr,
	}

	resp, err := client.RegisterMailbox(ctxReq, req)
	if err != nil {
		log.Fatalf("Mailbox: Could not register with Nameserver: %v", err)
	}
	if resp.GetSuccess() {
		log.Printf("Mailbox: Successfully registered '%s' with Nameserver: %s", username, resp.GetMessage())
	} else {
		log.Fatalf("Mailbox: Failed to register '%s' with Nameserver: %s", username, resp.GetMessage())
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

	if msg.Recipient == "" {
		return nil, status.Errorf(codes.InvalidArgument, "recipient cannot be empty")
	}

	s.userInboxes[msg.Recipient] = append(s.userInboxes[msg.Recipient], msg)
	log.Printf("Mailbox for '%s': Received new mail from '%s' (Subject: %s)",
		msg.Recipient, msg.Sender, msg.Subject)

	return &proto.ReceiveMailResponse{
		Success: true,
		Message: "Mail received successfully",
	}, nil
}

// GetMail implements proto.MailboxServer
// It retrieves all messages for a given user and then clears their inbox
func (s *server) GetMail(ctx context.Context, req *proto.GetMailRequest) (*proto.GetMailResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	username := req.GetUsername()
	if username == "" {
		return nil, status.Errorf(codes.InvalidArgument, "username cannot be empty")
	}

	messages, found := s.userInboxes[username]
	if !found || len(messages) == 0 {
		log.Printf("Mailbox for '%s': No new mail to retrieve", username)
		return &proto.GetMailResponse{Messages: []*proto.MailMessage{}}, nil
	}

	// Create a copy of message to return
	msgsToReturn := make([]*proto.MailMessage, len(messages))
	copy(msgsToReturn, messages)

	// Clear the inbox for the user after retrieval
	s.userInboxes[username] = []*proto.MailMessage{}
	log.Printf("Mailbox for '%s': Retrieved %d messages and cleared inbox", username, len(messages))

	return &proto.GetMailResponse{Messages: msgsToReturn}, nil
}
