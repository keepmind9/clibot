package cli

import "time"

// ACPTransportType represents the ACP transport type
type ACPTransportType string

const (
	ACPTransportStdio ACPTransportType = "stdio"
	ACPTransportTCP   ACPTransportType = "tcp"
	ACPTransportUnix  ACPTransportType = "unix"
)

// ACPAdapterConfig configuration for ACP adapter
type ACPAdapterConfig struct {
	// Request timeout duration
	RequestTimeout time.Duration `yaml:"request_timeout"`
	// Environment variables for ACP server process
	Env map[string]string `yaml:"env"`
}
