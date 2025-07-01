package transferserver

import (
	"GoDissys/common"
	"GoDissys/proto/proto"
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// server is used to implement proto.TransferServerServer
type server struct {
	proto.UnimplementedTransferServerServer
	nameserverClient proto.NameserverClient
}

// NewServer creates a new TransferServer instance
func NewServer(nameserverClient proto.NameserverClient) *server {
	return &server{
		nameserverClient: nameserverClient,
	}
}

// StartTransferServer starts the gRPC server for the TransferServer
func StartTransferServer() {
	// Connect to Nameserver to get its client
	nameserverDialCtx, nameserverDialCancel := context.WithTimeout(context.Background(), time.Second*5)
	defer nameserverDialCancel()
	nameserverConn, err := grpc.DialContext(nameserverDialCtx, common.NameserverAddr, grpc.WithInsecure()) // Insecure for practice
	if err != nil {
		log.Fatalf("TransferServer: Could not connect to Nameserver at %s: %v", common.NameserverAddr, err)
	}
	defer nameserverConn.Close() // This will close when StartTransferServer exits

	nameserverClient := proto.NewNameserverClient(nameserverConn)

	lis, err := net.Listen("tcp", common.TransferServerAddr)
	if err != nil {
		log.Fatalf("TransferServer failed to listen: %v", err)
	}
	s := grpc.NewServer()
	transferServerService := NewServer(nameserverClient)
	proto.RegisterTransferServerServer(s, transferServerService)
	log.Printf("TransferServer listening on %s", common.TransferServerAddr)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("TransferServer failed to serve: %v", err)
	}
}

// SendMail implements proto.TransferServerServer.
// It receives a mail message from a client, looks up the recipient's mailbox,
// and forwards the message to the appropriate mailbox.
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

	// 2. Connect to recipient's Mailbox and send the message
	recipientDialCtx, recipientDialCancel := context.WithTimeout(context.Background(), time.Second*5)
	defer recipientDialCancel()
	conn, err := grpc.DialContext(recipientDialCtx, recipientMailboxAddr, grpc.WithInsecure()) // Insecure for practice, use TLS in production
	if err != nil {
		log.Printf("TransferServer: Could not connect to recipient mailbox at %s: %v", recipientMailboxAddr, err)
		return nil, status.Errorf(codes.Internal, "failed to connect to recipient mailbox: %v", err)
	}
	defer conn.Close()

	mailboxClient := proto.NewMailboxClient(conn)

	sendToMailboxCtx, sendToMailboxCancel := context.WithTimeout(context.Background(), time.Second*5)
	defer sendToMailboxCancel()

	receiveMailReq := &proto.ReceiveMailRequest{Message: msg}
	receiveMailResp, err := mailboxClient.ReceiveMail(sendToMailboxCtx, receiveMailReq)
	if err != nil {
		log.Printf("TransferServer: Error sending mail to mailbox '%s': %v", recipientMailboxAddr, err)
		return nil, status.Errorf(codes.Internal, "failed to send mail to recipient mailbox: %v", err)
	}

	if receiveMailResp.GetSuccess() {
		log.Printf("TransferServer: Mail successfully delivered to '%s' (Mailbox: %s)", msg.RecipientEmail, recipientMailboxAddr)
		return &proto.SendMailResponse{Success: true, Message: "Mail sent successfully"}, nil
	} else {
		log.Printf("TransferServer: Mail delivery to '%s' failed: %s", msg.RecipientEmail, receiveMailResp.GetMessage())
		return &proto.SendMailResponse{Success: false, Message: "Mail delivery failed"}, nil
	}
}
