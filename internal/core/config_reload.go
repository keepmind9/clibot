package core

import (
	"fmt"
	"reflect"

	"github.com/keepmind9/clibot/internal/logger"
	"github.com/sirupsen/logrus"
)

// ReloadConfig loads config from the given path and atomically swaps it in.
// Only runtime-safe fields take effect immediately; startup-baked fields
// (logging, hook port, bot credentials, CLI adapters, static sessions) require a restart.
func (e *Engine) ReloadConfig(configPath string) error {
	newConfig, err := LoadConfig(configPath)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"config_file": configPath,
			"error":       err,
		}).Error("config-reload-failed")
		return fmt.Errorf("config reload failed: %w", err)
	}

	applyConfigDefaults(newConfig)

	oldConfig := e.config.Swap(newConfig)

	e.configAdapter.SetConfig(newConfig)
	e.proxyMgr.ClearCache()

	logReloadSummary(oldConfig, newConfig)
	return nil
}

// applyConfigDefaults mirrors the defaults applied in NewEngine.
func applyConfigDefaults(config *Config) {
	if config.Session.MaxDynamicSessions == 0 {
		config.Session.MaxDynamicSessions = 50
	}
	if config.Session.IdleTimeout == "" {
		config.Session.IdleTimeout = DefaultIdleTimeout
	}
	for name, botCfg := range config.Bots {
		if botCfg.MaxMessageLength == 0 && botCfg.Enabled {
			botCfg.MaxMessageLength = DefaultMaxMessageLength
			config.Bots[name] = botCfg
		}
	}
}

// logReloadSummary compares old and new config, logging what was reloaded
// and what requires a restart to take effect.
func logReloadSummary(old, new_ *Config) {
	var reloaded []string
	var needsRestart []string

	if !reflect.DeepEqual(old.Security, new_.Security) {
		reloaded = append(reloaded, "security")
	}
	if old.Session != new_.Session {
		reloaded = append(reloaded, "session")
	}
	if old.Proxy != new_.Proxy {
		reloaded = append(reloaded, "proxy")
	}
	if !reflect.DeepEqual(old.SessionTemplates, new_.SessionTemplates) {
		reloaded = append(reloaded, "session_templates")
	}
	for name, newBot := range new_.Bots {
		if oldBot, ok := old.Bots[name]; ok {
			if oldBot.MaxMessageLength != newBot.MaxMessageLength {
				reloaded = append(reloaded, fmt.Sprintf("bots.%s.max_message_length", name))
			}
		} else {
			needsRestart = append(needsRestart, fmt.Sprintf("bots.%s (added)", name))
		}
	}
	for name := range old.Bots {
		if _, ok := new_.Bots[name]; !ok {
			needsRestart = append(needsRestart, fmt.Sprintf("bots.%s (removed)", name))
		}
	}

	if old.Logging != new_.Logging {
		needsRestart = append(needsRestart, "logging")
	}
	if old.HookServer.Port != new_.HookServer.Port {
		needsRestart = append(needsRestart, "hook_server.port")
	}
	if !reflect.DeepEqual(old.Sessions, new_.Sessions) {
		needsRestart = append(needsRestart, "sessions")
	}
	if !reflect.DeepEqual(old.CLIAdapters, new_.CLIAdapters) {
		needsRestart = append(needsRestart, "cli_adapters")
	}
	for name, newBot := range new_.Bots {
		if oldBot, ok := old.Bots[name]; ok {
			if oldBot.Token != newBot.Token || oldBot.AppID != newBot.AppID || oldBot.AppSecret != newBot.AppSecret {
				needsRestart = append(needsRestart, fmt.Sprintf("bots.%s.credentials", name))
			}
		}
	}

	logger.WithFields(logrus.Fields{
		"reloaded":      reloaded,
		"needs_restart": needsRestart,
	}).Info("config-reloaded")

	for _, field := range needsRestart {
		logger.WithField("field", field).Warn("config-reload: requires restart to take effect")
	}
}
