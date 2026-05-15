package core

import (
	"sync/atomic"

	"github.com/keepmind9/clibot/internal/proxy"
)

// CoreConfigAdapter wraps core.Config to implement proxy.ConfigProvider
// This allows ProxyManager to access configuration without creating circular dependency
type CoreConfigAdapter struct {
	config atomic.Pointer[Config]
}

func NewCoreConfigAdapter(config *Config) *CoreConfigAdapter {
	a := &CoreConfigAdapter{}
	a.config.Store(config)
	return a
}

// SetConfig atomically updates the config pointer. Used during hot-reload.
func (a *CoreConfigAdapter) SetConfig(config *Config) {
	a.config.Store(config)
}

func (a *CoreConfigAdapter) get() *Config {
	return a.config.Load()
}

func (a *CoreConfigAdapter) GetGlobalProxyEnabled() bool {
	return a.get().Proxy.Enabled
}

func (a *CoreConfigAdapter) GetGlobalProxyURL() string {
	return a.get().Proxy.URL
}

func (a *CoreConfigAdapter) GetGlobalProxyUsername() string {
	return a.get().Proxy.Username
}

func (a *CoreConfigAdapter) GetGlobalProxyPassword() string {
	return a.get().Proxy.Password
}

func (a *CoreConfigAdapter) GetBotProxyEnabled(botType string) bool {
	if botConfig, exists := a.get().Bots[botType]; exists && botConfig.Proxy != nil {
		return botConfig.Proxy.Enabled
	}
	return false
}

func (a *CoreConfigAdapter) GetBotProxyURL(botType string) string {
	if botConfig, exists := a.get().Bots[botType]; exists && botConfig.Proxy != nil {
		return botConfig.Proxy.URL
	}
	return ""
}

func (a *CoreConfigAdapter) GetBotProxyUsername(botType string) string {
	if botConfig, exists := a.get().Bots[botType]; exists && botConfig.Proxy != nil {
		return botConfig.Proxy.Username
	}
	return ""
}

func (a *CoreConfigAdapter) GetBotProxyPassword(botType string) string {
	if botConfig, exists := a.get().Bots[botType]; exists && botConfig.Proxy != nil {
		return botConfig.Proxy.Password
	}
	return ""
}

var _ proxy.ConfigProvider = (*CoreConfigAdapter)(nil)
