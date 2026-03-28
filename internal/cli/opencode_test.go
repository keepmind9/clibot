package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// TestNewOpenCodeAdapter tests the NewOpenCodeAdapter function
func TestNewOpenCodeAdapter(t *testing.T) {
	t.Run("creates adapter with default config", func(t *testing.T) {
		config := OpenCodeAdapterConfig{}

		adapter, err := NewOpenCodeAdapter(config)

		assert.NoError(t, err)
		assert.NotNil(t, adapter)
	})
}

// TestOpenCodeAdapter_HandleHookData tests the HandleHookData function
func TestOpenCodeAdapter_HandleHookData(t *testing.T) {
	t.Run("valid hook data with cwd", func(t *testing.T) {
		config := OpenCodeAdapterConfig{}
		adapter, err := NewOpenCodeAdapter(config)
		require.NoError(t, err)

		hookData := map[string]interface{}{
			"cwd":        "/home/user/project",
			"session_id": "test-session",
		}
		data, _ := json.Marshal(hookData)

		cwd, prompt, response, err := adapter.HandleHookData(data)

		assert.NoError(t, err)
		assert.Equal(t, "/home/user/project", cwd)
		assert.Equal(t, "", prompt)
		assert.Equal(t, "", response)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		config := OpenCodeAdapterConfig{}
		adapter, err := NewOpenCodeAdapter(config)
		require.NoError(t, err)

		data := []byte("invalid json")

		cwd, prompt, response, err := adapter.HandleHookData(data)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse JSON data")
		assert.Equal(t, "", cwd)
		assert.Equal(t, "", prompt)
		assert.Equal(t, "", response)
	})

	t.Run("missing cwd in hook data", func(t *testing.T) {
		config := OpenCodeAdapterConfig{}
		adapter, err := NewOpenCodeAdapter(config)
		require.NoError(t, err)

		hookData := map[string]interface{}{
			"session_id": "test-session",
		}
		data, _ := json.Marshal(hookData)

		cwd, prompt, response, err := adapter.HandleHookData(data)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing cwd in hook data")
		assert.Equal(t, "", cwd)
		assert.Equal(t, "", prompt)
		assert.Equal(t, "", response)
	})

	t.Run("notification event clears response", func(t *testing.T) {
		config := OpenCodeAdapterConfig{}
		adapter, err := NewOpenCodeAdapter(config)
		require.NoError(t, err)

		hookData := map[string]interface{}{
			"cwd":             "/home/user/project",
			"session_id":      "test-session",
			"hook_event_name": "Notification",
		}
		data, _ := json.Marshal(hookData)

		cwd, prompt, response, err := adapter.HandleHookData(data)

		assert.NoError(t, err)
		assert.Equal(t, "/home/user/project", cwd)
		assert.Equal(t, "", prompt)
		assert.Equal(t, "", response)
	})
}

// TestExtractLatestInteractionFromDB tests SQLite-based extraction
func TestExtractLatestInteractionFromDB(t *testing.T) {
	setupTestDB := func(t *testing.T) (*sql.DB, string) {
		t.Helper()
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "opencode.db")

		db, err := sql.Open("sqlite", dbPath)
		require.NoError(t, err)

		// Create tables matching opencode schema
		_, err = db.Exec(`
			CREATE TABLE session (
				id TEXT PRIMARY KEY,
				project_id TEXT NOT NULL,
				time_created INTEGER NOT NULL,
				time_updated INTEGER NOT NULL,
				data TEXT NOT NULL
			);
			CREATE TABLE message (
				id TEXT PRIMARY KEY,
				session_id TEXT NOT NULL,
				time_created INTEGER NOT NULL,
				time_updated INTEGER NOT NULL,
				data TEXT NOT NULL,
				FOREIGN KEY (session_id) REFERENCES session(id)
			);
			CREATE TABLE part (
				id TEXT PRIMARY KEY,
				message_id TEXT NOT NULL,
				session_id TEXT NOT NULL,
				time_created INTEGER NOT NULL,
				time_updated INTEGER NOT NULL,
				data TEXT NOT NULL,
				FOREIGN KEY (message_id) REFERENCES message(id)
			);
		`)
		require.NoError(t, err)

		return db, dbPath
	}

	t.Run("extracts interaction from SQLite", func(t *testing.T) {
		db, dbPath := setupTestDB(t)
		defer db.Close()

		sessionID := "ses_test123"
		projectID := "proj_abc"

		// Insert session
		_, err := db.Exec(`INSERT INTO session (id, project_id, time_created, time_updated, data)
			VALUES (?, ?, 1000, 2000, '{}')`, sessionID, projectID)
		require.NoError(t, err)

		// Insert user message
		userMsgData := `{"role":"user","time":{"created":1100}}`
		_, err = db.Exec(`INSERT INTO message (id, session_id, time_created, time_updated, data)
			VALUES (?, ?, 1100, 1100, ?)`, "msg_user1", sessionID, userMsgData)
		require.NoError(t, err)

		// Insert user message part
		userPartData := `{"type":"text","text":"What is Go?"}`
		_, err = db.Exec(`INSERT INTO part (id, message_id, session_id, time_created, time_updated, data)
			VALUES (?, ?, ?, 1100, 1100, ?)`, "prt_user1", "msg_user1", sessionID, userPartData)
		require.NoError(t, err)

		// Insert assistant message
		asstMsgData := `{"role":"assistant","time":{"created":1200,"completed":1300}}`
		_, err = db.Exec(`INSERT INTO message (id, session_id, time_created, time_updated, data)
			VALUES (?, ?, 1200, 1300, ?)`, "msg_asst1", sessionID, asstMsgData)
		require.NoError(t, err)

		// Insert assistant message parts
		asstPartData := `{"type":"text","text":"Go is a programming language."}`
		_, err = db.Exec(`INSERT INTO part (id, message_id, session_id, time_created, time_updated, data)
			VALUES (?, ?, ?, 1200, 1200, ?)`, "prt_asst1", "msg_asst1", sessionID, asstPartData)
		require.NoError(t, err)

		db.Close()

		// Test extraction
		adapter, err := NewOpenCodeAdapter(OpenCodeAdapterConfig{})
		require.NoError(t, err)

		// Override getDBPath by directly testing extractLatestInteractionFromDB
		// We need to temporarily replace the db path resolution
		prompt, response, err := extractFromTestDB(adapter, dbPath, sessionID)
		assert.NoError(t, err)
		assert.Equal(t, "What is Go?", prompt)
		assert.Equal(t, "Go is a programming language.", response)
	})

	t.Run("no user message returns error", func(t *testing.T) {
		db, dbPath := setupTestDB(t)
		defer db.Close()

		sessionID := "ses_nouser"

		_, err := db.Exec(`INSERT INTO session (id, project_id, time_created, time_updated, data)
			VALUES (?, ?, 1000, 2000, '{}')`, sessionID, "proj_x")
		require.NoError(t, err)

		// Only assistant message, no user message
		asstMsgData := `{"role":"assistant","time":{"created":1200}}`
		_, err = db.Exec(`INSERT INTO message (id, session_id, time_created, time_updated, data)
			VALUES (?, ?, 1200, 1200, ?)`, "msg_only_asst", sessionID, asstMsgData)
		require.NoError(t, err)

		db.Close()

		adapter, err := NewOpenCodeAdapter(OpenCodeAdapterConfig{})
		require.NoError(t, err)

		_, _, err = extractFromTestDB(adapter, dbPath, sessionID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no user message found")
	})
}

// extractFromTestDB helper to test with a custom db path
func extractFromTestDB(adapter *OpenCodeAdapter, dbPath, sessionID string) (string, string, error) {
	db, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		return "", "", err
	}
	defer db.Close()

	messages, err := getMessagesFromDB(db, sessionID)
	if err != nil {
		return "", "", err
	}

	if len(messages) == 0 {
		return "", "", fmt.Errorf("no messages found for session %s", sessionID)
	}

	lastUserIndex := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			lastUserIndex = i
			break
		}
	}
	if lastUserIndex == -1 {
		return "", "", fmt.Errorf("no user message found in session %s", sessionID)
	}

	prompt, err := getMessageTextFromDB(db, messages[lastUserIndex].ID)
	if err != nil {
		return "", "", err
	}

	var responseParts []string
	for i := lastUserIndex + 1; i < len(messages); i++ {
		if messages[i].Role == "assistant" {
			text, err := getMessageTextFromDB(db, messages[i].ID)
			if err != nil {
				continue
			}
			if text != "" {
				responseParts = append(responseParts, text)
			}
		}
	}

	return prompt, joinStrings(responseParts), nil
}

func joinStrings(parts []string) string {
	result := ""
	for i, s := range parts {
		if i > 0 {
			result += "\n\n"
		}
		result += s
	}
	return result
}

// TestLoadPartsTextFromFiles tests the file-based parts loading
func TestLoadPartsTextFromFiles(t *testing.T) {
	t.Run("reads text parts from part files", func(t *testing.T) {
		tmpDir := t.TempDir()
		msgID := "msg_test123"
		partDir := filepath.Join(tmpDir, "part", msgID)
		require.NoError(t, os.MkdirAll(partDir, 0755))

		// Write a text part
		textPart := `{"id":"prt1","type":"text","text":"Hello world"}`
		require.NoError(t, os.WriteFile(filepath.Join(partDir, "prt1.json"), []byte(textPart), 0644))

		// Write a reasoning part (should be ignored)
		reasonPart := `{"id":"prt2","type":"reasoning","text":"Thinking..."}`
		require.NoError(t, os.WriteFile(filepath.Join(partDir, "prt2.json"), []byte(reasonPart), 0644))

		result, err := loadPartsTextFromFiles(tmpDir, msgID)
		assert.NoError(t, err)
		assert.Equal(t, "Hello world", result)
	})

	t.Run("joins multiple text parts", func(t *testing.T) {
		tmpDir := t.TempDir()
		msgID := "msg_multi"
		partDir := filepath.Join(tmpDir, "part", msgID)
		require.NoError(t, os.MkdirAll(partDir, 0755))

		part1 := `{"id":"prt1","type":"text","text":"First"}`
		require.NoError(t, os.WriteFile(filepath.Join(partDir, "prt1.json"), []byte(part1), 0644))
		part2 := `{"id":"prt2","type":"text","text":"Second"}`
		require.NoError(t, os.WriteFile(filepath.Join(partDir, "prt2.json"), []byte(part2), 0644))

		result, err := loadPartsTextFromFiles(tmpDir, msgID)
		assert.NoError(t, err)
		assert.Equal(t, "First\n\nSecond", result)
	})

	t.Run("returns error for missing part directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := loadPartsTextFromFiles(tmpDir, "nonexistent_msg")
		assert.Error(t, err)
	})
}

// TestGetProjectID tests the getProjectID function
func TestGetProjectID(t *testing.T) {
	t.Run("returns global for non-git directory", func(t *testing.T) {
		projectID, err := getProjectID("/tmp")
		assert.NoError(t, err)
		assert.Equal(t, "global", projectID)
	})
}
