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
	// Default idle timeout - max time without any activity before cancelling (5 minutes)
	defaultACPIdleTimeout = 5 * time.Minute

	// Default max total timeout - absolute maximum time for a request (1 hour)
	defaultACPMaxTotalTimeout = 1 * time.Hour

	// Connection ready timeout (30 seconds)
	acpConnectionReadyTimeout = 30 * time.Second

	// NewSession configuration
	acpNewSessionTimeout    = 60 * time.Second // per attempt
	acpNewSessionMaxRetries = 3                // maximum attempts
	acpNewSessionRetryDelay = 2 * time.Second  // between attempts

	// Connection stabilize delay after establishing connection (500ms)
	acpConnectionStabilizeDelay = 500 * time.Millisecond

	// Remote dial timeout (10 seconds)
	acpDialTimeout = 60 * time.Second

	// Poll interval for polling mode (1 second)
	acpPollInterval = 1 * time.Second

	// Activity check interval - how often to check for idle timeout (30 seconds)
	acpActivityCheckInterval = 30 * time.Second
)

// ACPAdapterConfig configuration for ACP adapter
type ACPAdapterConfig struct {
	// Idle timeout - max time without any activity before cancelling request
	// Default: 5 minutes. If the agent is actively working (sending updates),
	// the request will continue. Only cancelled if there's no activity for this duration.
	IdleTimeout time.Duration `yaml:"idle_timeout"`

	// Max total timeout - absolute maximum time for a request regardless of activity
	// Default: 1 hour. This is a hard limit to prevent truly hung requests.
	MaxTotalTimeout time.Duration `yaml:"max_total_timeout"`

	// Environment variables for ACP server process
	Env map[string]string `yaml:"env"`

	// Deprecated: Use IdleTimeout instead
	// Kept for backward compatibility
	RequestTimeout time.Duration `yaml:"request_timeout"`
}
