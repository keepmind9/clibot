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
	}
	engine.sessions["static1"] = &Session{
		Name:      "static1",
		CLIType:   "codex-stdio",
		IsDynamic: false,
	}
	engine.sessionMu.Unlock()

	engine.persistSessions()

	data, err := os.ReadFile(filepath.Join(tmpDir, "sessions.json"))
	require.NoError(t, err)

	var sessions []*Session
	require.NoError(t, json.Unmarshal(data, &sessions))

	assert.Len(t, sessions, 1)
	assert.Equal(t, "dyn1", sessions[0].Name)
	assert.True(t, sessions[0].IsDynamic)
}

func TestPersistSessions_RemovesFileWhenEmpty(t *testing.T) {
	engine := newTestEngine()
	tmpDir := t.TempDir()
	engine.config.DataDir = tmpDir

	// Create file first
	path := filepath.Join(tmpDir, "sessions.json")
	os.WriteFile(path, []byte("[]"), 0644)

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

	sessions := []*Session{
		{
			Name:         "restored",
			CLIType:      "codex-stdio",
			WorkDir:      "/tmp",
			StartCmd:     "codex-stdio",
			IsDynamic:    true,
			CreatedBy:    "testbot:user1",
			LastActiveAt: time.Now(),
		},
	}
	data, _ := json.Marshal(sessions)
	os.WriteFile(filepath.Join(tmpDir, "sessions.json"), data, 0644)

	engine.loadPersistedSessions()

	engine.sessionMu.RLock()
	defer engine.sessionMu.RUnlock()
	s, exists := engine.sessions["restored"]
	assert.True(t, exists, "alive session should be restored")
	assert.NotNil(t, s)
	assert.Equal(t, StateIdle, s.State)
}

func TestLoadPersistedSessions_SkipsMissingAdapter(t *testing.T) {
	engine := newTestEngine()
	// No adapter registered for "unknown-stdio"

	tmpDir := t.TempDir()
	engine.config.DataDir = tmpDir

	sessions := []*Session{
		{Name: "orphan", CLIType: "unknown-stdio", IsDynamic: true},
	}
	data, _ := json.Marshal(sessions)
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

	sessions := []*Session{
		{Name: "conflict", CLIType: "codex-stdio", IsDynamic: true},
	}
	data, _ := json.Marshal(sessions)
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
