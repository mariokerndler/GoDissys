package main

import (
	"GoDissys/client"
	"GoDissys/mailbox"
	"GoDissys/nameserver"
	"GoDissys/transferserver"
	"log"
	"time"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// Start Nameserver in a goroutine
	go nameserver.StartNameserver()
	time.Sleep(time.Millisecond * 500) // Give Nameserver a moment to start

	// Start Mailbox for user "alice" in a goroutine
	go mailbox.StartMailbox()
	time.Sleep(time.Millisecond * 500) // Give Mailbox a moment to start

	// Register "alice" (whose mailbox is running locally on MailboxPort) with the Nameserver
	// This call connects to Nameserver from the Mailbox component.
	mailbox.RegisterMailboxWithNameserver("alice")
	mailbox.RegisterMailboxWithNameserver("bob") // For demonstration, let's also register 'bob' to the same local mailbox.
	// In a real system, bob would have his own mailbox service.
	time.Sleep(time.Millisecond * 500) // Give registration a moment to propagate

	// Start TransferServer in a goroutine
	go transferserver.StartTransferServer()
	time.Sleep(time.Millisecond * 500) // Give TransferServer a moment to start

	log.Println("\n--- System initialized. Running client operations... ---\n")

	// --- Client Operations ---

	// Client sends mail to "alice"
	log.Println("Client: Sending mail from 'bob' to 'alice'...")
	client.SendMail("bob", "alice", "Hello from Bob", "Hi Alice, this is a test email from Bob!")
	time.Sleep(time.Second * 2) // Give time for mail to be transferred

	// Client sends another mail to "alice"
	log.Println("\nClient: Sending another mail from 'charlie' to 'alice'...")
	client.SendMail("charlie", "alice", "Meeting Reminder", "Don't forget our meeting tomorrow at 10 AM.")
	time.Sleep(time.Second * 2) // Give time for mail to be transferred

	// Client attempts to send mail to an unknown recipient "diana"
	log.Println("\nClient: Attempting to send mail to 'diana' (unknown recipient)...")
	client.SendMail("alice", "diana", "Test to unknown", "This email should not be delivered.")
	time.Sleep(time.Second * 2)

	// Client for "alice" retrieves mail
	log.Println("\nClient: Alice checking her mail...")
	client.GetMail("alice")
	time.Sleep(time.Second * 2) // Wait a bit

	// Client for "alice" checks mail again (should be empty now)
	log.Println("\nClient: Alice checking her mail again (should be empty)...")
	client.GetMail("alice")
	time.Sleep(time.Second * 2)

	// Keep the main goroutine alive to allow servers to run
	log.Println("\n--- All operations complete. Press Ctrl+C to exit. ---")
	select {} // Block forever
}
