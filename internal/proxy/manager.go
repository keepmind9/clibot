package proxy

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/keepmind9/clibot/internal/core"
	"golang.org/x/net/proxy"
)

// ProxyManager manages HTTP clients with proxy support
// It implements three-tier proxy priority: bot-level > global > environment variables
type ProxyManager struct {
	config  *core.Config
	mu      sync.RWMutex
	clients map[string]*http.Client
}

// NewProxyManager creates a new ProxyManager instance
func NewProxyManager(config *core.Config) *ProxyManager {
	return &ProxyManager{
		config:  config,
		clients: make(map[string]*http.Client),
	}
}

// GetHTTPClient returns an HTTP client for the specified bot type
// It caches clients to reuse connections and improve performance
func (pm *ProxyManager) GetHTTPClient(botType string) (*http.Client, error) {
	// Check cache first
	pm.mu.RLock()
	if client, exists := pm.clients[botType]; exists {
		pm.mu.RUnlock()
		return client, nil
	}
	pm.mu.RUnlock()

	// Resolve proxy configuration based on priority
	proxyConfig, err := pm.resolveProxyConfig(botType)
	if err != nil {
		return nil, err
	}

	// Create new HTTP client
	client, err := pm.createClient(proxyConfig)
	if err != nil {
		return nil, err
	}

	// Cache the client
	pm.mu.Lock()
	pm.clients[botType] = client
	pm.mu.Unlock()

	return client, nil
}

// GetProxyURL returns the proxy URL for the specified bot type
// Returns "env://HTTP_PROXY" if using environment variables
func (pm *ProxyManager) GetProxyURL(botType string) string {
	proxyConfig, _ := pm.resolveProxyConfig(botType)
	if proxyConfig == nil {
		return "env://HTTP_PROXY"
	}
	return proxyConfig.URL
}

// ClearCache removes all cached HTTP clients
// Useful for configuration reloads or testing
func (pm *ProxyManager) ClearCache() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.clients = make(map[string]*http.Client)
}

// resolveProxyConfig resolves proxy configuration with three-tier priority:
// 1. Bot-level proxy (highest priority)
// 2. Global proxy
// 3. No proxy (use environment variables)
func (pm *ProxyManager) resolveProxyConfig(botType string) (*core.ProxyConfig, error) {
	// 1. Bot-level proxy (highest priority)
	if botConfig, exists := pm.config.Bots[botType]; exists {
		if botConfig.Proxy != nil && botConfig.Proxy.Enabled {
			return botConfig.Proxy, nil
		}
	}

	// 2. Global proxy
	if pm.config.Proxy.Enabled {
		return &pm.config.Proxy, nil
	}

	// 3. No proxy (use environment variables)
	return nil, nil
}

// createClient creates an HTTP client with the specified proxy configuration
func (pm *ProxyManager) createClient(proxyConfig *core.ProxyConfig) (*http.Client, error) {
	if proxyConfig == nil {
		// Use environment variables (HTTP_PROXY, HTTPS_PROXY, NO_PROXY)
		return &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
			},
		}, nil
	}

	// Parse proxy URL
	proxyURL, err := url.Parse(proxyConfig.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}

	// Add authentication if provided
	if proxyConfig.Username != "" {
		proxyURL.User = url.UserPassword(proxyConfig.Username, proxyConfig.Password)
	}

	// Create transport based on proxy type
	transport, err := pm.createTransport(proxyConfig, proxyURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}, nil
}

// createTransport creates an HTTP transport for the specified proxy type
func (pm *ProxyManager) createTransport(cfg *core.ProxyConfig, proxyURL *url.URL) (*http.Transport, error) {
	var transport *http.Transport

	switch cfg.Type {
	case "http", "https":
		// HTTP/HTTPS proxy uses standard HTTP CONNECT method
		// This works for both HTTP and HTTPS requests
		transport = &http.Transport{
			Proxy:               http.ProxyURL(proxyURL),
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
			ForceAttemptHTTP2:   true,
		}

	case "socks5":
		// SOCKS5 proxy uses the golang.org/x/net/proxy package
		var auth *proxy.Auth
		if cfg.Username != "" {
			auth = &proxy.Auth{
				User:     cfg.Username,
				Password: cfg.Password,
			}
		}

		dialer, err := proxy.SOCKS5("tcp", proxyURL.Host, auth, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("SOCKS5 proxy error: %w", err)
		}

		transport = &http.Transport{
			DialContext:         dialer.(proxy.ContextDialer).DialContext,
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		}

	default:
		return nil, fmt.Errorf("unsupported proxy type: %s", cfg.Type)
	}

	return transport, nil
}
