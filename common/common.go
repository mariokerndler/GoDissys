package common

// Define port numbers for each service
const (
	NameserverPort     = "50051"
	TransferServerPort = "50053"

	// Define specific ports for mailboxes per domain
	EarthMailboxPort  = "50054"
	SaturnMailboxPort = "50055"

	// Localhost address for simplicity
	NameserverAddr     = "localhost:" + NameserverPort
	TransferServerAddr = "localhost:" + TransferServerPort
	EarthMailboxAddr   = "localhost:" + EarthMailboxPort
	SaturnMailboxAddr  = "localhost:" + SaturnMailboxPort
)
