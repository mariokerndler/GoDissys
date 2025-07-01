package nameserver

import (
	"GoDissys/common"
	"GoDissys/proto/proto"
	"context"
	"log"
	"net"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// server is used to implement proto.NameserverServer
type server struct {
	proto.UnimplementedNameserverServer

	// mailboxes maps usernames to their mailbox address (e.g., "localhost:50052")
	mailboxes map[string]string
	mu        sync.RWMutex // Mutex to protect the mailboxes map
}

// NewServer creates a new Nameserver instance
func NewServer() *server {
	return &server{
		mailboxes: make(map[string]string),
	}
}

// StartNameserver starts the gRPC server for the Nameserver
func StartNameserver() {
	lis, err := net.Listen("tcp", common.NameserverAddr)
	if err != nil {
		log.Fatalf("Nameserver failed to listen: %v", err)
	}
	s := grpc.NewServer()
	nameserverService := NewServer()
	proto.RegisterNameserverServer(s, nameserverService)
	log.Printf("Nameserver listening on %s", common.NameserverAddr)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Nameserver failed to server: %v", err)
	}

}

// RegisterMailbox implements proto.NameserverServer
// It registers a user's mailbox address with the nameserver
func (s *server) RegisterMailbox(ctx context.Context, req *proto.RegisterMailboxRequest) (*proto.RegisterMailboxResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	username := req.GetUsername()
	mailboxAddr := req.GetMailboxAddress()

	if username == "" || mailboxAddr == "" {
		return nil, status.Errorf(codes.InvalidArgument, "username and mailbox address cannot be empty")
	}

	if _, exists := s.mailboxes[username]; exists {
		log.Printf("Nameserver: User '%s' already registered, updating address to '%s'", username, mailboxAddr)
	} else {
		log.Printf("Nameserver: Registering user '%s' with mailbox at '%s'", username, mailboxAddr)
	}
	s.mailboxes[username] = mailboxAddr

	return &proto.RegisterMailboxResponse{
		Success: true,
		Message: "Mailbox registered successfully",
	}, nil
}

// LookupMailbox implements proto.NameserverServer
// It looks up the mailbox address for a given user
func (s *server) LookupMailbox(ctx context.Context, req *proto.LookupMailboxRequest) (*proto.LookupMailboxResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	username := req.GetUsername()
	if username == "" {
		return nil, status.Errorf(codes.InvalidArgument, "username cannot be empty")
	}

	addr, found := s.mailboxes[username]
	if !found {
		log.Printf("Nameserver: Mailbox for user '%s' not found", username)
		return &proto.LookupMailboxResponse{Found: false, MailboxAddress: ""}, nil
	}

	log.Printf("Nameserver: Found mailbox for user '%s' at '%s'", username, addr)
	return &proto.LookupMailboxResponse{Found: true, MailboxAddress: addr}, nil
}
