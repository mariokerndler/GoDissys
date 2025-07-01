package common

// Define port numbers for each service
const (
	NameserverPort     = "50051"
	MailboxPort        = "50052"
	TransferServerPort = "50053"

	// Localhost address for simplicity
	NameserverAddr     = "localhost:" + NameserverPort
	MailboxAddr        = "localhost:" + MailboxPort
	TransferServerAddr = "localhost:" + TransferServerPort
)
