package client

import (
	"GoDissys/mailbox"
	"GoDissys/proto/proto"
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
)

// Config holds the necessary addresses for the client to connect to services
type Config struct {
	NameserverAddr     string
	TransferServerAddr string
	Mailboxes          map[string]struct {
		Domain string
		Addr   string
	}
}

// currentClientState holds the state of the logged-in client
type currentClientState struct {
	EmailAddress   string
	MailboxAddress string
}

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

func StartCLI(cfg Config) {
	scanner := bufio.NewScanner(os.Stdin)
	var currentState currentClientState

	fmt.Println("\n--- Distributed Mail Client CLI ---")
	fmt.Println("Commands:")
	fmt.Println("  signup <your_email> <your_domain_mailbox_alias> - Register your email (e.g., alice@earth.com earth)")
	fmt.Println("  login <your_email> - Log in to manage your mail (e.g., alice@earth.com)")
	fmt.Println("  send <recipient_email> <subject> <body_text> - Send an email")
	fmt.Println("  get - Retrieve your mail")
	fmt.Println("  whoami - Show current logged-in user")
	fmt.Println("  exit - Quit the client")
	fmt.Print("> ")

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) == 0 {
			fmt.Print("> ")
			continue
		}

		command := strings.ToLower(parts[0])

		switch command {
		case "signup":
			if len(parts) != 3 {
				fmt.Println("Usage: signup <your_email> <your_domain_mailbox_alias>")
				fmt.Println("Example: signup alice@earth.com earth")
				break
			}
			email := parts[1]
			domainAlias := parts[2]
			mailboxConfig, ok := cfg.Mailboxes[getDomainFromEmail(email)]
			if !ok || mailboxConfig.Domain != domainAlias {
				fmt.Printf("Error: Mailbox configuration for domain '%s' (alias '%s') not found in config.json.\n", getDomainFromEmail(email), domainAlias)
				break
			}
			log.Printf("Attempting to sign up %s with mailbox at %s (Nameserver: %s)", email, mailboxConfig.Addr, cfg.NameserverAddr)
			// Call the mailbox's registration function
			mailbox.RegisterMailboxWithNameserver(cfg.NameserverAddr, email, mailboxConfig.Addr)
			fmt.Printf("Signup attempt for %s completed. You can now try to login.\n", email)

		case "login":
			if len(parts) != 2 {
				fmt.Println("Usage: login <your_email>")
				fmt.Println("Example: login alice@earth.com")
				break
			}
			email := parts[1]
			mailboxConfig, ok := cfg.Mailboxes[getDomainFromEmail(email)]
			if !ok {
				fmt.Printf("Error: Mailbox configuration for domain '%s' not found in config.json. Please signup first.\n", getDomainFromEmail(email))
				break
			}
			currentState.EmailAddress = email
			currentState.MailboxAddress = mailboxConfig.Addr
			fmt.Printf("Logged in as: %s\n", currentState.EmailAddress)

		case "send":
			if currentState.EmailAddress == "" {
				fmt.Println("Error: Please log in first using the 'login' command.")
				break
			}
			if len(parts) < 4 {
				fmt.Println("Usage: send <recipient_email> <subject> <body_text>")
				fmt.Println("Example: send bob@saturn.com 'Meeting' 'Let's meet tomorrow.'")
				break
			}
			recipientEmail := parts[1]
			subject := parts[2]
			body := strings.Join(parts[3:], " ")
			SendMail(cfg.TransferServerAddr, currentState.EmailAddress, recipientEmail, subject, body)

		case "get":
			if currentState.EmailAddress == "" {
				fmt.Println("Error: Please log in first using the 'login' command.")
				break
			}
			GetMail(currentState.EmailAddress, currentState.MailboxAddress)

		case "whoami":
			if currentState.EmailAddress == "" {
				fmt.Println("Not logged in.")
			} else {
				fmt.Printf("Currently logged in as: %s (Mailbox: %s)\n", currentState.EmailAddress, currentState.MailboxAddress)
			}

		case "exit":
			fmt.Println("Exiting client.")
			return

		default:
			fmt.Println("Unknown command. Type 'help' for available commands.")
		}
		fmt.Print("> ")
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading input: %v", err)
	}
}

// Helper function to extract domain from an email address
func getDomainFromEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) == 2 {
		return parts[1]
	}
	return "" // Invalid email format
}
