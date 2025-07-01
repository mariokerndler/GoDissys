package nameserver

import (
	"GoDissys/common"
	"GoDissys/proto/proto"
	"context"
	"fmt"
	"log"
	"net"
	"strings"
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

	// responsibleDomains store the domains this Nameserver is responsible for
	responsibleDomains map[string]bool
}

// NewServer creates a new Nameserver instance
func NewServer(domains []string) *server {
	rd := make(map[string]bool)
	for _, d := range domains {
		rd[d] = true
	}
	return &server{
		mailboxes:          make(map[string]string),
		responsibleDomains: rd,
	}
}

// StartNameserver starts the gRPC server for the Nameserver
func StartNameserver(domains ...string) {
	lis, err := net.Listen("tcp", common.NameserverAddr)
	if err != nil {
		log.Fatalf("Nameserver failed to listen: %v", err)
	}
	s := grpc.NewServer()
	nameserverService := NewServer(domains)
	proto.RegisterNameserverServer(s, nameserverService)
	log.Printf("Nameserver listening on %s, responsible for domains: %v", common.NameserverAddr, domains)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Nameserver failed to server: %v", err)
	}

}

// RegisterMailbox implements proto.NameserverServer.
// It registers a user's full email address with their mailbox address,
// but only if the email's domain is managed by this Nameserver.
func (s *server) RegisterMailbox(ctx context.Context, req *proto.RegisterMailboxRequest) (*proto.RegisterMailboxResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	emailAddress := req.GetEmailAddress()
	mailboxAddr := req.GetMailboxAddress()

	if emailAddress == "" || mailboxAddr == "" {
		return nil, status.Errorf(codes.InvalidArgument, "email address and mailbox address cannot be empty")
	}

	// Extract domain from email address
	parts := strings.Split(emailAddress, "@")
	if len(parts) != 2 {
		return nil, status.Errorf(codes.InvalidArgument, "invalid email address format: %s", emailAddress)
	}
	domain := parts[1]

	// Check if this Nameserver is responsible for the domain
	if !s.responsibleDomains[domain] {
		log.Printf("Nameserver: Registration rejected for '%s'. Domain '%s' is not managed by this Nameserver.", emailAddress, domain)
		return &proto.RegisterMailboxResponse{
			Success: false,
			Message: fmt.Sprintf("Domain '%s' is not managed by this Nameserver.", domain),
		}, nil
	}

	if _, exists := s.mailboxes[emailAddress]; exists {
		log.Printf("Nameserver: Email '%s' already registered, updating address to '%s'", emailAddress, mailboxAddr)
	} else {
		log.Printf("Nameserver: Registering email '%s' with mailbox at '%s'", emailAddress, mailboxAddr)
	}
	s.mailboxes[emailAddress] = mailboxAddr

	return &proto.RegisterMailboxResponse{Success: true, Message: "Mailbox registered successfully"}, nil
}

// LookupMailbox implements proto.NameserverServer
// It looks up the mailbox address for a given user
func (s *server) LookupMailbox(ctx context.Context, req *proto.LookupMailboxRequest) (*proto.LookupMailboxResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	username := req.GetEmailAddress()
	if username == "" {
		return nil, status.Errorf(codes.InvalidArgument, "email address cannot be empty")
	}

	addr, found := s.mailboxes[username]
	if !found {
		log.Printf("Nameserver: Mailbox for user '%s' not found", username)
		return &proto.LookupMailboxResponse{Found: false, MailboxAddress: ""}, nil
	}

	log.Printf("Nameserver: Found mailbox for email '%s' at '%s'", username, addr)
	return &proto.LookupMailboxResponse{Found: true, MailboxAddress: addr}, nil
}
