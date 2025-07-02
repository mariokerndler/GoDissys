package client

import (
	"GoDissys/proto/proto"
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
)

// SendMail connects to the TransferServer and sends a mail message.
func SendMail(transferServerAddr, senderEmail, recipientEmail, subject, body string) {
	transferDialCtx, transferDialCancel := context.WithTimeout(context.Background(), time.Second*5)
	defer transferDialCancel()
	conn, err := grpc.DialContext(transferDialCtx, transferServerAddr, grpc.WithInsecure()) // Insecure for practice
	if err != nil {
		log.Fatalf("Client: Could not connect to TransferServer at %s: %v", transferServerAddr, err)
	}
	defer conn.Close()

	client := proto.NewTransferServerClient(conn)

	ctxReq, cancelReq := context.WithTimeout(context.Background(), time.Second*10)
	defer cancelReq()

	msg := &proto.MailMessage{
		SenderEmail:    senderEmail,
		RecipientEmail: recipientEmail,
		Subject:        subject,
		Body:           body,
		Timestamp:      time.Now().Unix(),
	}

	req := &proto.SendMailRequest{Message: msg}

	resp, err := client.SendMail(ctxReq, req)
	if err != nil {
		log.Printf("Client: Error sending mail: %v", err)
		return
	}

	if resp.GetSuccess() {
		log.Printf("Client: Mail sent successfully to '%s': %s", recipientEmail, resp.GetMessage())
	} else {
		log.Printf("Client: Failed to send mail to '%s': %s", recipientEmail, resp.GetMessage())
	}
}

// GetMail connects to a specific Mailbox (e.g., the user's own) and retrieves messages.
func GetMail(emailAddress, mailboxAddr string) {
	mailboxDialCtx, mailboxDialCancel := context.WithTimeout(context.Background(), time.Second*5)
	defer mailboxDialCancel()
	conn, err := grpc.DialContext(mailboxDialCtx, mailboxAddr, grpc.WithInsecure()) // Insecure for practice
	if err != nil {
		log.Fatalf("Client: Could not connect to Mailbox at %s for '%s': %v", mailboxAddr, emailAddress, err)
	}
	defer conn.Close()

	client := proto.NewMailboxClient(conn)

	ctxReq, cancelReq := context.WithTimeout(context.Background(), time.Second*5)
	defer cancelReq()

	req := &proto.GetMailRequest{EmailAddress: emailAddress}

	resp, err := client.GetMail(ctxReq, req)
	if err != nil {
		log.Printf("Client: Error getting mail for '%s': %v", emailAddress, err)
		return
	}

	messages := resp.GetMessages()
	if len(messages) == 0 {
		log.Printf("Client for '%s': No new messages.", emailAddress)
		return
	}

	log.Printf("Client for '%s': Retrieved %d messages:", emailAddress, len(messages))
	for i, msg := range messages {
		fmt.Printf("--- Message %d ---\n", i+1)
		fmt.Printf("From: %s\n", msg.SenderEmail)
		fmt.Printf("Subject: %s\n", msg.Subject)
		fmt.Printf("Timestamp: %s\n", time.Unix(msg.Timestamp, 0).Format(time.RFC822))
		fmt.Printf("Body:\n%s\n", msg.Body)
		fmt.Println("-----------------")
	}
}
