package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPersistSessions_WritesFile(t *testing.T) {
	engine := newTestEngine()
	engine.RegisterCLIAdapter("codex-stdio", &mockCLIAdapter{})

	tmpDir := t.TempDir()
	engine.config.DataDir = tmpDir

	engine.sessionMu.Lock()
	engine.sessions["dyn1"] = &Session{
		Name:         "dyn1",
		CLIType:      "codex-stdio",
		WorkDir:      "/tmp",
		StartCmd:     "codex-stdio",
		IsDynamic:    true,
		CreatedBy:    "testbot:user1",
		LastActiveAt: time.Now(),
		LastChannel:  &BotChannelRef{Platform: "testbot", Channel: "ch1"},
		Env:          map[string]string{"FOO": "bar"},
	}
	engine.sessions["static1"] = &Session{
		Name:      "static1",
		CLIType:   "codex-stdio",
		IsDynamic: false,
	}
	engine.userSessions["testbot:user1"] = "dyn1"
	engine.sessionMu.Unlock()

	engine.persistSessions()

	data, err := os.ReadFile(filepath.Join(tmpDir, "sessions.json"))
	require.NoError(t, err)

	var state sessionState
	require.NoError(t, json.Unmarshal(data, &state))

	assert.Len(t, state.Sessions, 1)
	assert.Equal(t, "dyn1", state.Sessions[0].Name)
	assert.True(t, state.Sessions[0].IsDynamic)
	assert.Equal(t, "dyn1", state.UserSessions["testbot:user1"])
}

func TestPersistSessions_RemovesFileWhenEmpty(t *testing.T) {
	engine := newTestEngine()
	tmpDir := t.TempDir()
	engine.config.DataDir = tmpDir

	// Create file first
	path := filepath.Join(tmpDir, "sessions.json")
	os.WriteFile(path, []byte("{}"), 0644)

	engine.sessionMu.Lock()
	engine.sessions["static1"] = &Session{Name: "static1", IsDynamic: false}
	engine.sessionMu.Unlock()

	engine.persistSessions()

	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err), "file should be removed when no dynamic sessions")
}

func TestLoadPersistedSessions_RestoresAlive(t *testing.T) {
	engine := newTestEngine()
	engine.RegisterCLIAdapter("codex-stdio", &mockCLIAdapter{})

	tmpDir := t.TempDir()
	engine.config.DataDir = tmpDir

	state := sessionState{
		Sessions: []*Session{
			{
				Name:         "restored",
				CLIType:      "codex-stdio",
				WorkDir:      "/tmp",
				StartCmd:     "codex-stdio",
				IsDynamic:    true,
				CreatedBy:    "testbot:user1",
				LastActiveAt: time.Now(),
				LastChannel:  &BotChannelRef{Platform: "testbot", Channel: "ch1"},
				Env:          map[string]string{"FOO": "bar"},
			},
		},
		UserSessions: map[string]string{"testbot:user1": "restored"},
	}
	data, _ := json.Marshal(state)
	os.WriteFile(filepath.Join(tmpDir, "sessions.json"), data, 0644)

	engine.loadPersistedSessions()

	engine.sessionMu.RLock()
	defer engine.sessionMu.RUnlock()
	s, exists := engine.sessions["restored"]
	assert.True(t, exists, "alive session should be restored")
	assert.NotNil(t, s)
	assert.Equal(t, StateIdle, s.State)

	// Verify sessionChannels restored from LastChannel
	ch, ok := engine.sessionChannels["restored"]
	assert.True(t, ok, "sessionChannels should be restored")
	assert.Equal(t, "testbot", ch.Platform)
	assert.Equal(t, "ch1", ch.Channel)

	// Verify userSessions restored from persisted state
	assert.Equal(t, "restored", engine.userSessions["testbot:user1"])
}

func TestLoadPersistedSessions_SkipsMissingAdapter(t *testing.T) {
	engine := newTestEngine()
	// No adapter registered for "unknown-stdio"

	tmpDir := t.TempDir()
	engine.config.DataDir = tmpDir

	state := sessionState{
		Sessions: []*Session{
			{Name: "orphan", CLIType: "unknown-stdio", IsDynamic: true},
		},
	}
	data, _ := json.Marshal(state)
	os.WriteFile(filepath.Join(tmpDir, "sessions.json"), data, 0644)

	engine.loadPersistedSessions()

	engine.sessionMu.RLock()
	defer engine.sessionMu.RUnlock()
	_, exists := engine.sessions["orphan"]
	assert.False(t, exists, "session with missing adapter should be skipped")
}

func TestLoadPersistedSessions_SkipsIfStaticExists(t *testing.T) {
	engine := newTestEngine()
	engine.RegisterCLIAdapter("codex-stdio", &mockCLIAdapter{})

	tmpDir := t.TempDir()
	engine.config.DataDir = tmpDir

	engine.sessionMu.Lock()
	engine.sessions["conflict"] = &Session{Name: "conflict", CLIType: "codex-stdio", IsDynamic: false}
	engine.sessionMu.Unlock()

	state := sessionState{
		Sessions: []*Session{
			{Name: "conflict", CLIType: "codex-stdio", IsDynamic: true},
		},
	}
	data, _ := json.Marshal(state)
	os.WriteFile(filepath.Join(tmpDir, "sessions.json"), data, 0644)

	engine.loadPersistedSessions()

	engine.sessionMu.RLock()
	defer engine.sessionMu.RUnlock()
	s := engine.sessions["conflict"]
	assert.False(t, s.IsDynamic, "static session should not be overwritten")
}

func TestLoadPersistedSessions_NoFile(t *testing.T) {
	engine := newTestEngine()
	engine.config.DataDir = t.TempDir()

	// Should not panic
	engine.loadPersistedSessions()

	engine.sessionMu.RLock()
	defer engine.sessionMu.RUnlock()
	assert.Empty(t, engine.sessions)
}

func TestLoadPersistedSessions_BackwardCompatOldFormat(t *testing.T) {
	engine := newTestEngine()
	engine.RegisterCLIAdapter("codex-stdio", &mockCLIAdapter{})

	tmpDir := t.TempDir()
	engine.config.DataDir = tmpDir

	// Write old format: bare array instead of sessionState object
	oldFormat := []*Session{
		{
			Name:         "old",
			CLIType:      "codex-stdio",
			IsDynamic:    true,
			LastActiveAt: time.Now(),
		},
	}
	data, _ := json.Marshal(oldFormat)
	os.WriteFile(filepath.Join(tmpDir, "sessions.json"), data, 0644)

	engine.loadPersistedSessions()

	engine.sessionMu.RLock()
	defer engine.sessionMu.RUnlock()
	_, exists := engine.sessions["old"]
	assert.True(t, exists, "old format should still be loadable")
}
