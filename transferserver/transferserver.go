package transferserver

import (
	"GoDissys/proto/proto"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	maxRetries     = 3                      // Maximum number of retries for mail delivery to mailbox
	initialBackoff = 500 * time.Millisecond // Initial delay before retrying
	maxBackoff     = 5 * time.Second        // Maximum delay between retries
)

// server is used to implement proto.TransferServerServer.
type server struct {
	proto.UnimplementedTransferServerServer
	nameserverClient proto.NameserverClient
}

// NewServer creates a new TransferServer instance.
func NewServer(nameserverClient proto.NameserverClient) *server {
	return &server{
		nameserverClient: nameserverClient,
	}
}

// StartTransferServer starts the gRPC server for the TransferServer.
func StartTransferServer(nameserverAddr, transferServerAddr string) {
	// Connect to Nameserver to get its client
	nameserverDialCtx, nameserverDialCancel := context.WithTimeout(context.Background(), time.Second*5)
	nameserverConn, err := grpc.DialContext(nameserverDialCtx, nameserverAddr, grpc.WithInsecure()) // Insecure for practice
	nameserverDialCancel()                                                                          // Ensure context is cancelled after DialContext returns

	if err != nil {
		log.Printf("TransferServer: Could not connect to Nameserver at %s: %v", nameserverAddr, err)
		return // Return instead of Fatalf
	}

	nameserverClient := proto.NewNameserverClient(nameserverConn)

	lis, err := net.Listen("tcp", transferServerAddr) // Use transferServerAddr
	if err != nil {
		log.Printf("TransferServer failed to listen on %s: %v", transferServerAddr, err)
		nameserverConn.Close() // Close client connection if listen fails
		return                 // Return instead of Fatalf
	}
	s := grpc.NewServer()
	transferServerService := NewServer(nameserverClient)
	proto.RegisterTransferServerServer(s, transferServerService)
	log.Printf("TransferServer listening on %s", transferServerAddr)

	// Goroutine to serve gRPC requests
	go func() {
		if err := s.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			log.Printf("TransferServer failed to serve: %v", err)
		}
	}()

	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit // Block until a signal is received
	log.Printf("TransferServer received shutdown signal. Shutting down gracefully...")
	s.GracefulStop() // Gracefully stop the gRPC server
	log.Println("TransferServer server stopped.")

	// Explicitly close the Nameserver client connection AFTER the server has stopped
	nameserverConn.Close()
}

// SendMail implements proto.TransferServerServer.
// It receives a mail message from a client, looks up the recipient's mailbox,
// and forwards the message to the appropriate mailbox with retry logic.
func (s *server) SendMail(ctx context.Context, req *proto.SendMailRequest) (*proto.SendMailResponse, error) {
	msg := req.GetMessage()
	if msg == nil {
		return nil, status.Errorf(codes.InvalidArgument, "mail message cannot be empty")
	}
	if msg.RecipientEmail == "" {
		return nil, status.Errorf(codes.InvalidArgument, "recipient email cannot be empty")
	}

	log.Printf("TransferServer: Received mail from '%s' for '%s' (Subject: %s)",
		msg.SenderEmail, msg.RecipientEmail, msg.Subject)

	// 1. Lookup recipient's mailbox address from Nameserver using the full email address
	lookupCtx, lookupCancel := context.WithTimeout(context.Background(), time.Second*5)
	defer lookupCancel()

	lookupReq := &proto.LookupMailboxRequest{EmailAddress: msg.RecipientEmail}
	lookupResp, err := s.nameserverClient.LookupMailbox(lookupCtx, lookupReq)
	if err != nil {
		log.Printf("TransferServer: Error looking up mailbox for '%s': %v", msg.RecipientEmail, err)
		return nil, status.Errorf(codes.Internal, "failed to lookup recipient mailbox: %v", err)
	}

	if !lookupResp.GetFound() {
		log.Printf("TransferServer: Recipient '%s' not found by Nameserver.", msg.RecipientEmail)
		return &proto.SendMailResponse{Success: false, Message: fmt.Sprintf("Recipient '%s' not found", msg.RecipientEmail)}, nil
	}

	recipientMailboxAddr := lookupResp.GetMailboxAddress()
	log.Printf("TransferServer: Found recipient '%s' at mailbox address '%s'", msg.RecipientEmail, recipientMailboxAddr)

	// 2. Establish connection to recipient's Mailbox once for all retry attempts
	recipientDialCtx, recipientDialCancel := context.WithTimeout(context.Background(), time.Second*5)
	conn, err := grpc.DialContext(recipientDialCtx, recipientMailboxAddr, grpc.WithInsecure()) // Insecure for practice, use TLS in production
	recipientDialCancel()                                                                      // Ensure context is cancelled after DialContext returns

	if err != nil {
		log.Printf("TransferServer: Initial connection to recipient mailbox at %s failed: %v", recipientMailboxAddr, err)
		return nil, status.Errorf(codes.Unavailable, "failed to connect to recipient mailbox: %v", err)
	}
	defer conn.Close() // Close connection when SendMail function exits

	mailboxClient := proto.NewMailboxClient(conn)

	// Loop for initial attempt + maxRetries retries
	var lastErr error
	backoff := initialBackoff
	for i := 0; i <= maxRetries; i++ { // Loop for initial attempt (i=0) + maxRetries additional retries
		log.Printf("TransferServer: Attempt %d/%d to deliver mail to '%s' at '%s'", i+1, maxRetries+1, msg.RecipientEmail, recipientMailboxAddr)

		sendToMailboxCtx, sendToMailboxCancel := context.WithTimeout(context.Background(), time.Second*5)
		receiveMailReq := &proto.ReceiveMailRequest{Message: msg}
		receiveMailResp, err := mailboxClient.ReceiveMail(sendToMailboxCtx, receiveMailReq)
		sendToMailboxCancel() // Ensure context is cancelled after RPC returns

		if err != nil {
			lastErr = fmt.Errorf("error sending mail to mailbox '%s': %v", recipientMailboxAddr, err)
			log.Printf("TransferServer: Mail delivery RPC failed: %v", lastErr)
			if i < maxRetries { // Only sleep if more retries are available
				time.Sleep(backoff)
				backoff *= 2 // Exponential backoff
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
			continue
		}

		if receiveMailResp.GetSuccess() {
			log.Printf("TransferServer: Mail successfully delivered to '%s' (Mailbox: %s)", msg.RecipientEmail, recipientMailboxAddr)
			return &proto.SendMailResponse{Success: true, Message: "Mail sent successfully"}, nil
		} else {
			lastErr = fmt.Errorf("mail delivery to '%s' failed: %s", msg.RecipientEmail, receiveMailResp.GetMessage())
			log.Printf("TransferServer: Mail delivery response indicated failure: %v", lastErr)
			if i < maxRetries { // Only sleep if more retries are available
				time.Sleep(backoff)
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
			continue
		}
	}

	// If we reach here, all retries failed
	log.Printf("TransferServer: All %d attempts to deliver mail to '%s' failed. Last error: %v", maxRetries+1, msg.RecipientEmail, lastErr)
	return &proto.SendMailResponse{Success: false, Message: fmt.Sprintf("Mail delivery failed after %d retries: %v", maxRetries, lastErr)}, nil
}
