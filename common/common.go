package common

import (
	"encoding/json"
	"fmt"
	"os"
)

// MailboxConfig holds configuration for a specific mailbox instance
type MailboxConfig struct {
	Domain string `json:"Domain"`
	Addr   string `json:"Addr"`
}

// Config holds the entire application configuration
type Config struct {
	NameserverAddr           string                   `json:"NameserverAddr"`
	TransferServerAddr       string                   `json:"TransferServerAddr"`
	Mailboxes                map[string]MailboxConfig `json:"Mailboxes"`
	NameserverManagedDomains []string                 `json:"NameserverManagedDomains"`
}

// LoadConfig reads the configuration from a JSON file.
func LoadConfig(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file '%s': %w", filePath, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config data from '%s': %w", filePath, err)
	}

	return &cfg, nil
}
