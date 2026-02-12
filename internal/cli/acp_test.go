package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewACPAdapter_DefaultConfig tests adapter creation with default config
func TestNewACPAdapter_DefaultConfig(t *testing.T) {
	config := ACPAdapterConfig{}

	adapter, err := NewACPAdapter(config)

	require.NoError(t, err)
	assert.NotNil(t, adapter)
	assert.Equal(t, 5*time.Minute, adapter.config.RequestTimeout)
	assert.NotNil(t, adapter.sessions)
}

// TestNewACPAdapter_CustomConfig tests adapter creation with custom config
func TestNewACPAdapter_CustomConfig(t *testing.T) {
	config := ACPAdapterConfig{
		RequestTimeout: 10 * time.Minute,
		Env: map[string]string{
			"TEST_VAR": "test_value",
		},
	}

	adapter, err := NewACPAdapter(config)

	require.NoError(t, err)
	assert.NotNil(t, adapter)
	assert.Equal(t, 10*time.Minute, adapter.config.RequestTimeout)
	assert.Equal(t, "test_value", adapter.config.Env["TEST_VAR"])
}

// TestACPAdapter_UseHook tests that ACP adapter doesn't use hook mode
func TestACPAdapter_UseHook(t *testing.T) {
	adapter, _ := NewACPAdapter(ACPAdapterConfig{})

	assert.False(t, adapter.UseHook(), "ACP adapter should not use hook mode")
}

// TestACPAdapter_GetPollInterval tests poll interval
func TestACPAdapter_GetPollInterval(t *testing.T) {
	adapter, _ := NewACPAdapter(ACPAdapterConfig{})

	assert.Equal(t, 1*time.Second, adapter.GetPollInterval())
}

// TestACPAdapter_GetStableCount tests stable count
func TestACPAdapter_GetStableCount(t *testing.T) {
	adapter, _ := NewACPAdapter(ACPAdapterConfig{})

	assert.Equal(t, 1, adapter.GetStableCount())
}

// TestACPAdapter_GetPollTimeout tests poll timeout
func TestACPAdapter_GetPollTimeout(t *testing.T) {
	config := ACPAdapterConfig{RequestTimeout: 3 * time.Minute}
	adapter, _ := NewACPAdapter(config)

	assert.Equal(t, 3*time.Minute, adapter.GetPollTimeout())
}

// TestACPAdapter_HandleHookData tests that hook data is not supported
func TestACPAdapter_HandleHookData(t *testing.T) {
	adapter, _ := NewACPAdapter(ACPAdapterConfig{})

	_, _, _, err := adapter.HandleHookData([]byte("test"))

	assert.Error(t, err, "ACP mode should return error for hook data")
	assert.Contains(t, err.Error(), "does not use hook data")
}

// TestACPAdapter_IsSessionAlive tests session alive check
func TestACPAdapter_IsSessionAlive(t *testing.T) {
	adapter, _ := NewACPAdapter(ACPAdapterConfig{})

	// Session doesn't exist
	assert.False(t, adapter.IsSessionAlive("nonexistent"))

	// Create a session (simulated - we won't actually start server)
	adapter.sessions["test"] = &acpSession{
		active: true,
	}
	assert.True(t, adapter.IsSessionAlive("test"))

	// Inactive session
	adapter.sessions["test2"] = &acpSession{
		active: false,
	}
	assert.False(t, adapter.IsSessionAlive("test2"))
}

// TestParseTransportURL tests transport URL parsing
func TestParseTransportURL(t *testing.T) {
	tests := []struct {
		url          string
		expected     ACPTransportType
		expectedAddr string
	}{
		{"stdio://", ACPTransportStdio, ""},
		{"", ACPTransportStdio, ""}, // Empty defaults to stdio
		{"tcp://127.0.0.1:9000", ACPTransportTCP, "127.0.0.1:9000"},
		{"unix:///tmp/acp.sock", ACPTransportUnix, "/tmp/acp.sock"},
		{"unix:///var/run/acp", ACPTransportUnix, "/var/run/acp"},
		{"invalid", ACPTransportStdio, ""}, // Invalid URL defaults to stdio
	}

	for _, tt := range tests {
		transportType, addr := parseTransportURL(tt.url)
		assert.Equal(t, tt.expected, transportType, "URL: %s", tt.url)
		assert.Equal(t, tt.expectedAddr, addr, "URL: %s", tt.url)
	}
}
