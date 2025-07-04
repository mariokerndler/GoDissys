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

	// Start TransferServer in a goroutine
	wg.Add(1)
	go func() {
		defer wg.Done() // Signal when this goroutine is done
		transferserver.StartTransferServer(cfg.NameserverAddr, cfg.TransferServerAddr)
	}()
	time.Sleep(time.Millisecond * 500) // Give TransferServer a moment to start

	log.Println("\n--- All services initialized. Starting client CLI... ---")

	// Start the client CLI in the main goroutine
	// The CLI will handle user interactions for signup, login, send, and get mail.
	// We need to pass the relevant parts of the config to the client CLI.
	clientConfig := client.Config{
		NameserverAddr:     cfg.NameserverAddr,
		TransferServerAddr: cfg.TransferServerAddr,
		Mailboxes: make(map[string]struct {
			Domain string
			Addr   string
		}),
	}
	for domain, mbCfg := range cfg.Mailboxes {
		clientConfig.Mailboxes[domain] = struct {
			Domain string
			Addr   string
		}{Domain: mbCfg.Domain, Addr: mbCfg.Addr}
	}

	client.StartCLI(clientConfig) // This call blocks until the user exits the CLI

	// After the client CLI exits, wait for all server goroutines to complete their graceful shutdown
	log.Println("Client CLI exited. Waiting for all services to stop...")
	wg.Wait()
	log.Println("All services have stopped.")
}
