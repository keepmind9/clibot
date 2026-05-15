package core

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minValidYAML returns a minimal valid config with the given overrides applied.
func minValidYAML(overrides string) []byte {
	base := `
sessions:
  - name: default
    cli_type: test-stdio
    work_dir: /tmp
bots:
  testbot:
    enabled: true
    token: "test-token"
    channel_id: "test-channel"
security:
  whitelist_enabled: false
session:
  max_dynamic_sessions: 10
  idle_timeout: "30m"
`
	if overrides != "" {
		return []byte(overrides)
	}
	return []byte(base)
}

func TestReloadConfig_Success(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yaml1 := []byte(`
sessions:
  - name: default
    cli_type: test-stdio
    work_dir: /tmp
bots:
  testbot:
    enabled: true
    token: "test-token"
    channel_id: "test-channel"
security:
  whitelist_enabled: false
  allowed_users:
    testbot: ["user1"]
session:
  max_dynamic_sessions: 10
  idle_timeout: "30m"
`)
	require.NoError(t, os.WriteFile(configPath, yaml1, 0644))

	config, err := LoadConfig(configPath)
	require.NoError(t, err)

	engine := NewEngine(config)
	engine.SetConfigPath(configPath)

	assert.Equal(t, 10, engine.cfg().Session.MaxDynamicSessions)
	assert.Equal(t, []string{"user1"}, engine.cfg().Security.AllowedUsers["testbot"])

	// Update config file with new values
	yaml2 := []byte(`
sessions:
  - name: default
    cli_type: test-stdio
    work_dir: /tmp
bots:
  testbot:
    enabled: true
    token: "test-token"
    channel_id: "test-channel"
security:
  whitelist_enabled: true
  allowed_users:
    testbot: ["user2", "user3"]
session:
  max_dynamic_sessions: 20
  idle_timeout: "1h"
`)
	require.NoError(t, os.WriteFile(configPath, yaml2, 0644))

	err = engine.ReloadConfig(configPath)
	require.NoError(t, err)

	assert.True(t, engine.cfg().Security.WhitelistEnabled)
	assert.Equal(t, []string{"user2", "user3"}, engine.cfg().Security.AllowedUsers["testbot"])
	assert.Equal(t, 20, engine.cfg().Session.MaxDynamicSessions)
	assert.Equal(t, "1h", engine.cfg().Session.IdleTimeout)
}

func TestReloadConfig_InvalidFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	require.NoError(t, os.WriteFile(configPath, minValidYAML(""), 0644))

	config, err := LoadConfig(configPath)
	require.NoError(t, err)

	engine := NewEngine(config)

	// Try reloading from nonexistent file
	err = engine.ReloadConfig(filepath.Join(tmpDir, "nonexistent.yaml"))
	assert.Error(t, err)

	// Original config should be unchanged
	assert.Equal(t, 10, engine.cfg().Session.MaxDynamicSessions)
}

func TestReloadConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	require.NoError(t, os.WriteFile(configPath, minValidYAML(""), 0644))

	config, err := LoadConfig(configPath)
	require.NoError(t, err)

	engine := NewEngine(config)

	// Write invalid YAML
	require.NoError(t, os.WriteFile(configPath, []byte("{{invalid yaml"), 0644))

	err = engine.ReloadConfig(configPath)
	assert.Error(t, err)

	// Original config should be unchanged
	assert.Equal(t, 10, engine.cfg().Session.MaxDynamicSessions)
}

func TestReloadConfig_ConcurrentRead(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	require.NoError(t, os.WriteFile(configPath, minValidYAML(""), 0644))

	config, err := LoadConfig(configPath)
	require.NoError(t, err)

	engine := NewEngine(config)

	var wg sync.WaitGroup
	var readErrors atomic.Int32

	// Start concurrent readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				cfg := engine.cfg()
				if cfg == nil {
					readErrors.Add(1)
				}
				_ = cfg.Session.MaxDynamicSessions
				_ = cfg.Security.WhitelistEnabled
			}
		}()
	}

	// Reload config concurrently
	newYaml := []byte(`
sessions:
  - name: default
    cli_type: test-stdio
    work_dir: /tmp
bots:
  testbot:
    enabled: true
    token: "test-token"
    channel_id: "test-channel"
security:
  whitelist_enabled: true
  allowed_users:
    testbot: ["user1"]
session:
  max_dynamic_sessions: 99
  idle_timeout: "1h"
`)
	require.NoError(t, os.WriteFile(configPath, newYaml, 0644))
	require.NoError(t, engine.ReloadConfig(configPath))

	wg.Wait()
	assert.Equal(t, int32(0), readErrors.Load())
	assert.Equal(t, 99, engine.cfg().Session.MaxDynamicSessions)
}

func TestReloadConfig_SessionTemplates(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yaml1 := []byte(`
sessions:
  - name: default
    cli_type: test-stdio
    work_dir: /tmp
bots:
  testbot:
    enabled: true
    token: "test-token"
    channel_id: "test-channel"
security:
  whitelist_enabled: false
session:
  max_dynamic_sessions: 10
  idle_timeout: "30m"
session_templates:
  my-template:
    cli_type: "claude-stdio"
    yolo: true
    env:
      KEY: "value1"
`)
	require.NoError(t, os.WriteFile(configPath, yaml1, 0644))

	config, err := LoadConfig(configPath)
	require.NoError(t, err)

	engine := NewEngine(config)
	assert.Equal(t, "value1", engine.cfg().SessionTemplates["my-template"].Env["KEY"])

	// Update templates
	yaml2 := []byte(`
sessions:
  - name: default
    cli_type: test-stdio
    work_dir: /tmp
bots:
  testbot:
    enabled: true
    token: "test-token"
    channel_id: "test-channel"
security:
  whitelist_enabled: false
session:
  max_dynamic_sessions: 10
  idle_timeout: "30m"
session_templates:
  my-template:
    cli_type: "claude-stdio"
    yolo: false
    env:
      KEY: "value2"
  new-template:
    cli_type: "codex-stdio"
`)
	require.NoError(t, os.WriteFile(configPath, yaml2, 0644))

	require.NoError(t, engine.ReloadConfig(configPath))
	assert.Equal(t, "value2", engine.cfg().SessionTemplates["my-template"].Env["KEY"])
	assert.False(t, engine.cfg().SessionTemplates["my-template"].Yolo)
	assert.NotNil(t, engine.cfg().SessionTemplates["new-template"])
}

func TestReloadConfig_IsUserAuthorized(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yaml1 := []byte(`
sessions:
  - name: default
    cli_type: test-stdio
    work_dir: /tmp
bots:
  testbot:
    enabled: true
    token: "test-token"
    channel_id: "test-channel"
security:
  whitelist_enabled: true
  allowed_users:
    testbot: ["user1"]
session:
  max_dynamic_sessions: 10
  idle_timeout: "30m"
`)
	require.NoError(t, os.WriteFile(configPath, yaml1, 0644))

	config, err := LoadConfig(configPath)
	require.NoError(t, err)

	engine := NewEngine(config)

	// user1 is authorized, user2 is not
	assert.True(t, engine.cfg().IsUserAuthorized("testbot", "user1"))
	assert.False(t, engine.cfg().IsUserAuthorized("testbot", "user2"))

	// Reload: swap whitelist to user2 + user3
	yaml2 := []byte(`
sessions:
  - name: default
    cli_type: test-stdio
    work_dir: /tmp
bots:
  testbot:
    enabled: true
    token: "test-token"
    channel_id: "test-channel"
security:
  whitelist_enabled: true
  allowed_users:
    testbot: ["user2", "user3"]
session:
  max_dynamic_sessions: 10
  idle_timeout: "30m"
`)
	require.NoError(t, os.WriteFile(configPath, yaml2, 0644))
	require.NoError(t, engine.ReloadConfig(configPath))

	// user1 is now denied, user2/user3 are allowed
	assert.False(t, engine.cfg().IsUserAuthorized("testbot", "user1"))
	assert.True(t, engine.cfg().IsUserAuthorized("testbot", "user2"))
	assert.True(t, engine.cfg().IsUserAuthorized("testbot", "user3"))

	// Reload: disable whitelist entirely
	yaml3 := []byte(`
sessions:
  - name: default
    cli_type: test-stdio
    work_dir: /tmp
bots:
  testbot:
    enabled: true
    token: "test-token"
    channel_id: "test-channel"
security:
  whitelist_enabled: false
session:
  max_dynamic_sessions: 10
  idle_timeout: "30m"
`)
	require.NoError(t, os.WriteFile(configPath, yaml3, 0644))
	require.NoError(t, engine.ReloadConfig(configPath))

	// All users now allowed
	assert.True(t, engine.cfg().IsUserAuthorized("testbot", "anyone"))
}

func TestReloadConfig_IsAdmin(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yaml1 := []byte(`
sessions:
  - name: default
    cli_type: test-stdio
    work_dir: /tmp
bots:
  testbot:
    enabled: true
    token: "test-token"
    channel_id: "test-channel"
security:
  whitelist_enabled: false
  admins:
    testbot: ["admin1"]
session:
  max_dynamic_sessions: 10
  idle_timeout: "30m"
`)
	require.NoError(t, os.WriteFile(configPath, yaml1, 0644))

	config, err := LoadConfig(configPath)
	require.NoError(t, err)

	engine := NewEngine(config)

	assert.True(t, engine.cfg().IsAdmin("testbot", "admin1"))
	assert.False(t, engine.cfg().IsAdmin("testbot", "admin2"))
	assert.False(t, engine.cfg().IsAdmin("otherbot", "admin1"))

	// Reload: change admin list
	yaml2 := []byte(`
sessions:
  - name: default
    cli_type: test-stdio
    work_dir: /tmp
bots:
  testbot:
    enabled: true
    token: "test-token"
    channel_id: "test-channel"
security:
  whitelist_enabled: false
  admins:
    testbot: ["admin2", "admin3"]
session:
  max_dynamic_sessions: 10
  idle_timeout: "30m"
`)
	require.NoError(t, os.WriteFile(configPath, yaml2, 0644))
	require.NoError(t, engine.ReloadConfig(configPath))

	assert.False(t, engine.cfg().IsAdmin("testbot", "admin1"))
	assert.True(t, engine.cfg().IsAdmin("testbot", "admin2"))
	assert.True(t, engine.cfg().IsAdmin("testbot", "admin3"))
}

func TestReloadConfig_FailurePreservesProxyCache(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	require.NoError(t, os.WriteFile(configPath, minValidYAML(""), 0644))

	config, err := LoadConfig(configPath)
	require.NoError(t, err)

	engine := NewEngine(config)

	// Populate proxy cache
	pm := engine.GetProxyManager()
	client1, err := pm.GetHTTPClient("testbot")
	require.NoError(t, err)
	require.NotNil(t, client1)

	// Corrupt the config file
	require.NoError(t, os.WriteFile(configPath, []byte("{{invalid"), 0644))

	err = engine.ReloadConfig(configPath)
	assert.Error(t, err)

	// Proxy cache should still have the cached client
	client2, err := pm.GetHTTPClient("testbot")
	require.NoError(t, err)
	assert.Same(t, client1, client2, "proxy cache should not be cleared on reload failure")
}
