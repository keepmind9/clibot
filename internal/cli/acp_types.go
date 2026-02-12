package cli

import "time"

// ACPTransportType represents the ACP transport type
type ACPTransportType string

const (
	ACPTransportStdio ACPTransportType = "stdio"
	ACPTransportTCP   ACPTransportType = "tcp"
	ACPTransportUnix  ACPTransportType = "unix"
)

// ACP adapter constants
const (
	// Default timeout for ACP requests (5 minutes)
	defaultACPRequestTimeout = 5 * time.Minute

	// Connection ready timeout (30 seconds)
	acpConnectionReadyTimeout = 30 * time.Second

	// NewSession configuration
	acpNewSessionTimeout     = 10 * time.Second // per attempt
	acpNewSessionMaxRetries  = 3                // maximum attempts
	acpNewSessionRetryDelay  = 2 * time.Second // between attempts

	// Connection stabilize delay after establishing connection (500ms)
	acpConnectionStabilizeDelay = 500 * time.Millisecond

	// Remote dial timeout (10 seconds)
	acpDialTimeout = 10 * time.Second

	// Poll interval for polling mode (1 second)
	acpPollInterval = 1 * time.Second
)

// ACPAdapterConfig configuration for ACP adapter
type ACPAdapterConfig struct {
	// Request timeout duration
	RequestTimeout time.Duration `yaml:"request_timeout"`
	// Environment variables for ACP server process
	Env map[string]string `yaml:"env"`
}
