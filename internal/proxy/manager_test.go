package proxy

import (
	"testing"

	"github.com/keepmind9/clibot/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestProxyManager_NoProxy_ReturnsClientWithEnvProxy(t *testing.T) {
	config := &core.Config{
		Proxy: core.ProxyConfig{
			Enabled: false,
		},
		Bots: map[string]core.BotConfig{},
	}

	pm := NewProxyManager(config)
	client, err := pm.GetHTTPClient("telegram")

	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.NotNil(t, client.Transport)
}

func TestProxyManager_GlobalProxy_ReturnsClientWithProxy(t *testing.T) {
	config := &core.Config{
		Proxy: core.ProxyConfig{
			Enabled: true,
			Type:    "http",
			URL:     "http://127.0.0.1:8080",
		},
		Bots: map[string]core.BotConfig{},
	}

	pm := NewProxyManager(config)
	client, err := pm.GetHTTPClient("telegram")

	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestProxyManager_BotLevelProxyOverridesGlobal(t *testing.T) {
	botProxy := &core.ProxyConfig{
		Enabled: true,
		Type:    "socks5",
		URL:     "socks5://127.0.0.1:1080",
	}

	config := &core.Config{
		Proxy: core.ProxyConfig{
			Enabled: true,
			Type:    "http",
			URL:     "http://127.0.0.1:8080",
		},
		Bots: map[string]core.BotConfig{
			"telegram": {
				Proxy: botProxy,
			},
		},
	}

	pm := NewProxyManager(config)
	proxyURL := pm.GetProxyURL("telegram")

	assert.Contains(t, proxyURL, "socks5://127.0.0.1:1080")
}

func TestProxyManager_ClientCaching(t *testing.T) {
	config := &core.Config{
		Proxy: core.ProxyConfig{
			Enabled: true,
			Type:    "http",
			URL:     "http://127.0.0.1:8080",
		},
		Bots: map[string]core.BotConfig{},
	}

	pm := NewProxyManager(config)

	// First call
	client1, err := pm.GetHTTPClient("telegram")
	assert.NoError(t, err)
	assert.NotNil(t, client1)

	// Second call should return cached client
	client2, err := pm.GetHTTPClient("telegram")
	assert.NoError(t, err)
	assert.Same(t, client1, client2)
}

func TestProxyManager_ClearCache(t *testing.T) {
	config := &core.Config{
		Proxy: core.ProxyConfig{
			Enabled: true,
			Type:    "http",
			URL:     "http://127.0.0.1:8080",
		},
		Bots: map[string]core.BotConfig{},
	}

	pm := NewProxyManager(config)

	// Create cached client
	client1, err := pm.GetHTTPClient("telegram")
	assert.NoError(t, err)
	assert.NotNil(t, client1)

	// Clear cache
	pm.ClearCache()

	// New client should be different
	client2, err := pm.GetHTTPClient("telegram")
	assert.NoError(t, err)
	assert.NotNil(t, client2)
	assert.NotSame(t, client1, client2)
}

func TestProxyManager_InvalidProxyURL(t *testing.T) {
	config := &core.Config{
		Proxy: core.ProxyConfig{
			Enabled: true,
			Type:    "http",
			URL:     "://invalid-url",
		},
		Bots: map[string]core.BotConfig{},
	}

	pm := NewProxyManager(config)
	_, err := pm.GetHTTPClient("telegram")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid proxy URL")
}

func TestProxyManager_UnsupportedProxyType(t *testing.T) {
	config := &core.Config{
		Proxy: core.ProxyConfig{
			Enabled: true,
			Type:    "ftp",
			URL:     "ftp://127.0.0.1:2121",
		},
		Bots: map[string]core.BotConfig{},
	}

	pm := NewProxyManager(config)
	_, err := pm.GetHTTPClient("telegram")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported proxy type")
}

func TestProxyManager_ProxyWithAuth(t *testing.T) {
	config := &core.Config{
		Proxy: core.ProxyConfig{
			Enabled:  true,
			Type:     "http",
			URL:      "http://127.0.0.1:8080",
			Username: "user",
			Password: "pass",
		},
		Bots: map[string]core.BotConfig{},
	}

	pm := NewProxyManager(config)
	client, err := pm.GetHTTPClient("telegram")

	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.NotNil(t, client.Transport)
}

func TestProxyManager_Socks5Proxy(t *testing.T) {
	config := &core.Config{
		Proxy: core.ProxyConfig{
			Enabled: true,
			Type:    "socks5",
			URL:     "socks5://127.0.0.1:1080",
		},
		Bots: map[string]core.BotConfig{},
	}

	pm := NewProxyManager(config)
	client, err := pm.GetHTTPClient("telegram")

	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.NotNil(t, client.Transport)
}

func TestProxyManager_Socks5WithAuth(t *testing.T) {
	config := &core.Config{
		Proxy: core.ProxyConfig{
			Enabled:  true,
			Type:     "socks5",
			URL:      "socks5://127.0.0.1:1080",
			Username: "user",
			Password: "pass",
		},
		Bots: map[string]core.BotConfig{},
	}

	pm := NewProxyManager(config)
	client, err := pm.GetHTTPClient("telegram")

	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.NotNil(t, client.Transport)
}
