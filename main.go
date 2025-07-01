package main

import (
	"GoDissys/client"
	"GoDissys/common"
	"GoDissys/mailbox"
	"GoDissys/nameserver"
	"GoDissys/transferserver"
	"log"
	"time"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// Start Nameserver in a goroutine, responsible for both earth.com and saturn.com
	// In a more complex setup, you might have separate Nameservers for each domain.
	go nameserver.StartNameserver("earth.com", "saturn.com")
	time.Sleep(time.Millisecond * 500) // Give Nameserver a moment to start

	// Start Mailbox for earth.com in a goroutine
	go mailbox.StartMailbox("earth", common.EarthMailboxPort)
	time.Sleep(time.Millisecond * 500) // Give Mailbox a moment to start

	// Start Mailbox for saturn.com in a goroutine
	go mailbox.StartMailbox("saturn", common.SaturnMailboxPort)
	time.Sleep(time.Millisecond * 500) // Give Mailbox a moment to start

	// Register users with their respective mailbox servers via the Nameserver
	// These should succeed as the Nameserver is configured for these domains.
	mailbox.RegisterMailboxWithNameserver("alice@earth.com", common.EarthMailboxAddr)
	mailbox.RegisterMailboxWithNameserver("bob@saturn.com", common.SaturnMailboxAddr)
	mailbox.RegisterMailboxWithNameserver("charlie@earth.com", common.EarthMailboxAddr)
	time.Sleep(time.Millisecond * 500) // Give registrations a moment to propagate

	// Attempt to register a user for an unmanaged domain (should fail)
	log.Println("\nAttempting to register 'diana@mars.com' (should be rejected by Nameserver)...")
	// For demonstration, we'll let it show the rejection.
	// In a real application, you'd likely check the response and handle the error gracefully
	// instead of potentially hitting a log.Fatalf in RegisterMailboxWithNameserver.
	// For this specific case, if the Nameserver rejects, the `RegisterMailboxWithNameserver`
	// function will `log.Fatalf`, stopping the program.
	// If you want to see the program continue, you would need to modify `RegisterMailboxWithNameserver`
	// to return an error instead of calling `log.Fatalf` and handle that error here.
	// mailbox.RegisterMailboxWithNameserver("diana@mars.com", "localhost:9999") // Uncomment to test rejection

	// Start TransferServer in a goroutine
	go transferserver.StartTransferServer()
	time.Sleep(time.Millisecond * 500) // Give TransferServer a moment to start

	log.Println("\n--- System initialized. Running client operations... ---")

	// --- Client Operations ---

	// Client sends mail from earth.com to saturn.com
	log.Println("Client: Sending mail from 'alice@earth.com' to 'bob@saturn.com'...")
	client.SendMail("alice@earth.com", "bob@saturn.com", "Hello from Earth!", "Hi Bob, this is a test email from Alice on Earth!")
	time.Sleep(time.Second * 2) // Give time for mail to be transferred

	// Client sends mail within earth.com
	log.Println("\nClient: Sending mail from 'charlie@earth.com' to 'alice@earth.com'...")
	client.SendMail("charlie@earth.com", "alice@earth.com", "Meeting Reminder (Earth)", "Alice, don't forget our meeting tomorrow.")
	time.Sleep(time.Second * 2) // Give time for mail to be transferred

	// Client sends mail from saturn.com to earth.com (simulated by client sending via the *single* TransferServer)
	log.Println("\nClient: Sending mail from 'bob@saturn.com' to 'charlie@earth.com'...")
	client.SendMail("bob@saturn.com", "charlie@earth.com", "Greetings from Saturn!", "Hey Charlie, hope you're doing well on Earth!")
	time.Sleep(time.Second * 2) // Give time for mail to be transferred

	// Client attempts to send mail to an unknown recipient "diana@mars.com"
	log.Println("\nClient: Attempting to send mail to 'diana@mars.com' (unknown recipient)...")
	client.SendMail("alice@earth.com", "diana@mars.com", "Test to unknown domain", "This email should not be delivered.")
	time.Sleep(time.Second * 2)

	// Client for "alice@earth.com" retrieves mail
	log.Println("\nClient: Alice@earth.com checking her mail...")
	client.GetMail("alice@earth.com", common.EarthMailboxAddr) // Alice connects to her earth.com mailbox
	time.Sleep(time.Second * 2)                                // Wait a bit

	// Client for "bob@saturn.com" retrieves mail
	log.Println("\nClient: Bob@saturn.com checking his mail...")
	client.GetMail("bob@saturn.com", common.SaturnMailboxAddr) // Bob connects to his saturn.com mailbox
	time.Sleep(time.Second * 2)

	// Client for "charlie@earth.com" retrieves mail
	log.Println("\nClient: Charlie@earth.com checking his mail...")
	client.GetMail("charlie@earth.com", common.EarthMailboxAddr) // Charlie connects to his earth.com mailbox
	time.Sleep(time.Second * 2)

	// Keep the main goroutine alive to allow servers to run
	log.Println("\n--- All operations complete. Press Ctrl+C to exit. ---")
	select {} // Block forever
}
