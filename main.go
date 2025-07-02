package main

import (
	"GoDissys/client"
	"GoDissys/common"
	"GoDissys/mailbox"
	"GoDissys/nameserver"
	"GoDissys/transferserver"
	"log"
	"sync"
	"time"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// Load configuration from file
	cfg, err := common.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	var wg sync.WaitGroup // Use WaitGroup to keep main goroutine alive until all servers are stopped

	// Start Nameserver in a goroutine
	wg.Add(1)
	go func() {
		defer wg.Done() // Signal when this goroutine is done
		nameserver.StartNameserver(cfg.NameserverAddr, cfg.NameserverManagedDomains...)
	}()
	time.Sleep(time.Millisecond * 500) // Give Nameserver a moment to start

	// Start Mailbox for earth.com in a goroutine
	earthMailboxConfig, ok := cfg.Mailboxes["earth.com"]
	if !ok {
		log.Fatalf("Earth.com mailbox configuration not found")
	}
	wg.Add(1)
	go func() {
		defer wg.Done() // Signal when this goroutine is done
		mailbox.StartMailbox(earthMailboxConfig.Domain, earthMailboxConfig.Addr)
	}()
	time.Sleep(time.Millisecond * 500) // Give Mailbox a moment to start

	// Start Mailbox for saturn.com in a goroutine
	saturnMailboxConfig, ok := cfg.Mailboxes["saturn.com"]
	if !ok {
		log.Fatalf("Saturn.com mailbox configuration not found")
	}
	wg.Add(1)
	go func() {
		defer wg.Done() // Signal when this goroutine is done
		mailbox.StartMailbox(saturnMailboxConfig.Domain, saturnMailboxConfig.Addr)
	}()
	time.Sleep(time.Millisecond * 500) // Give Mailbox a moment to start

	// Register users with their respective mailbox servers via the Nameserver
	mailbox.RegisterMailboxWithNameserver(cfg.NameserverAddr, "alice@earth.com", earthMailboxConfig.Addr)
	mailbox.RegisterMailboxWithNameserver(cfg.NameserverAddr, "bob@saturn.com", saturnMailboxConfig.Addr)
	mailbox.RegisterMailboxWithNameserver(cfg.NameserverAddr, "charlie@earth.com", earthMailboxConfig.Addr)
	time.Sleep(time.Millisecond * 500) // Give registrations a moment to propagate

	// Attempt to register a user for an unmanaged domain (should fail)
	log.Println("\nAttempting to register 'diana@mars.com' (should be rejected by Nameserver)...")
	// This call will still cause a fatal error if the Nameserver rejects it due to log.Fatalf in RegisterMailboxWithNameserver.
	// If you want to see the program continue, you would need to modify `RegisterMailboxWithNameserver`
	// to return an error instead of calling `log.Fatalf` and handle that error here.
	// For demonstration purposes, we'll leave it as is to highlight the rejection.
	// mailbox.RegisterMailboxWithNameserver(cfg.NameserverAddr, "diana@mars.com", "localhost:9999") // Uncomment to test rejection

	// Start TransferServer in a goroutine
	wg.Add(1)
	go func() {
		defer wg.Done() // Signal when this goroutine is done
		transferserver.StartTransferServer(cfg.NameserverAddr, cfg.TransferServerAddr)
	}()
	time.Sleep(time.Millisecond * 500) // Give TransferServer a moment to start

	log.Println("\n--- System initialized. Running client operations... ---")

	// --- Client Operations ---

	// Client sends mail from earth.com to saturn.com
	log.Println("Client: Sending mail from 'alice@earth.com' to 'bob@saturn.com'...")
	client.SendMail(cfg.TransferServerAddr, "alice@earth.com", "bob@saturn.com", "Hello from Earth!", "Hi Bob, this is a test email from Alice on Earth!")
	time.Sleep(time.Second * 2) // Give time for mail to be transferred

	// Client sends mail within earth.com
	log.Println("\nClient: Sending mail from 'charlie@earth.com' to 'alice@earth.com'...")
	client.SendMail(cfg.TransferServerAddr, "charlie@earth.com", "alice@earth.com", "Meeting Reminder (Earth)", "Alice, don't forget our meeting tomorrow.")
	time.Sleep(time.Second * 2) // Give time for mail to be transferred

	// Client sends mail from saturn.com to earth.com (simulated by client sending via the *single* TransferServer)
	log.Println("\nClient: Sending mail from 'bob@saturn.com' to 'charlie@earth.com'...")
	client.SendMail(cfg.TransferServerAddr, "bob@saturn.com", "charlie@earth.com", "Greetings from Saturn!", "Hey Charlie, hope you're doing well on Earth!")
	time.Sleep(time.Second * 2) // Give time for mail to be transferred

	// Client attempts to send mail to an unknown recipient "diana@mars.com"
	log.Println("\nClient: Attempting to send mail to 'diana@mars.com' (unknown recipient)...")
	client.SendMail(cfg.TransferServerAddr, "alice@earth.com", "diana@mars.com", "Test to unknown domain", "This email should not be delivered.")
	time.Sleep(time.Second * 2)

	// Client for "alice@earth.com" retrieves mail
	log.Println("\nClient: Alice@earth.com checking her mail...")
	client.GetMail("alice@earth.com", earthMailboxConfig.Addr) // Alice connects to her earth.com mailbox
	time.Sleep(time.Second * 2)                                // Wait a bit

	// Client for "bob@saturn.com" retrieves mail
	log.Println("\nClient: Bob@saturn.com checking his mail...")
	client.GetMail("bob@saturn.com", saturnMailboxConfig.Addr) // Bob connects to his saturn.com mailbox
	time.Sleep(time.Second * 2)

	// Client for "charlie@earth.com" retrieves mail
	log.Println("\nClient: Charlie@earth.com checking his mail...")
	client.GetMail("charlie@earth.com", earthMailboxConfig.Addr) // Charlie connects to his earth.com mailbox
	time.Sleep(time.Second * 2)

	// Wait for all servers to gracefully shut down (on Ctrl+C)
	log.Println("\n--- All operations complete. Press Ctrl+C to exit. ---")
	wg.Wait() // Block main goroutine until all goroutines in WaitGroup are done
	log.Println("All services have stopped.")
}
