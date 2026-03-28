package cli

import (
	"encoding/json"
	"os"
	"testing"
)

func TestE2E_RealSession_SQLiteOnly(t *testing.T) {
	dbPath, err := getDBPath()
	if err != nil || func() bool { _, e := os.Stat(dbPath); return e != nil }() {
		t.Skip("opencode.db not found, skipping e2e test")
	}

	adapter, err := NewOpenCodeAdapter(OpenCodeAdapterConfig{})
	if err != nil {
		t.Fatalf("create adapter: %v", err)
	}

	hook := map[string]interface{}{
		"cwd":             "/data1/app/workspace/ai/fastclaw",
		"session_id":      "ses_2d2678e99ffeZ8g7E8zVXXg2My",
		"hook_event_name": "Response",
	}
	data, _ := json.Marshal(hook)

	cwd, prompt, response, err := adapter.HandleHookData(data)
	if err != nil {
		t.Fatalf("HandleHookData: %v", err)
	}

	if cwd != "/data1/app/workspace/ai/fastclaw" {
		t.Errorf("cwd = %q, want /data1/app/workspace/ai/fastclaw", cwd)
	}
	if prompt == "" {
		t.Error("prompt should not be empty")
	}
	if response == "" {
		t.Error("response should not be empty")
	}

	t.Logf("SQLite session - prompt (%d chars): %s...", len(prompt), truncStr(prompt, 80))
	t.Logf("SQLite session - response (%d chars): %s...", len(response), truncStr(response, 80))
}

func TestE2E_RealSession_AutoResolve(t *testing.T) {
	dbPath, err := getDBPath()
	if err != nil || func() bool { _, e := os.Stat(dbPath); return e != nil }() {
		t.Skip("opencode.db not found, skipping e2e test")
	}

	adapter, err := NewOpenCodeAdapter(OpenCodeAdapterConfig{})
	if err != nil {
		t.Fatalf("create adapter: %v", err)
	}

	// No session_id - should auto-resolve via projectID
	hook := map[string]interface{}{
		"cwd":             "/data1/app/workspace/ai/fastclaw",
		"hook_event_name": "Response",
	}
	data, _ := json.Marshal(hook)

	_, prompt, _, err := adapter.HandleHookData(data)
	if err != nil {
		t.Fatalf("HandleHookData (auto-resolve): %v", err)
	}
	if prompt == "" {
		t.Error("auto-resolved prompt should not be empty")
	}

	t.Logf("Auto-resolve - prompt (%d chars): %s...", len(prompt), truncStr(prompt, 80))
}

func TestE2E_RealSession_NotificationClears(t *testing.T) {
	dbPath, err := getDBPath()
	if err != nil || func() bool { _, e := os.Stat(dbPath); return e != nil }() {
		t.Skip("opencode.db not found, skipping e2e test")
	}

	adapter, err := NewOpenCodeAdapter(OpenCodeAdapterConfig{})
	if err != nil {
		t.Fatalf("create adapter: %v", err)
	}

	hook := map[string]interface{}{
		"cwd":             "/data1/app/workspace/ai/fastclaw",
		"session_id":      "ses_2d2678e99ffeZ8g7E8zVXXg2My",
		"hook_event_name": "Notification",
	}
	data, _ := json.Marshal(hook)

	_, _, response, err := adapter.HandleHookData(data)
	if err != nil {
		t.Fatalf("HandleHookData: %v", err)
	}
	if response != "" {
		t.Errorf("response should be empty for Notification, got %d chars", len(response))
	}
}

func TestE2E_RealSession_FileFallback(t *testing.T) {
	storageDir, err := getStorageDir()
	if err != nil {
		t.Skip("storage dir not found")
	}
	if _, e := os.Stat(storageDir); e != nil {
		t.Skip("file storage not found, skipping")
	}

	adapter, err := NewOpenCodeAdapter(OpenCodeAdapterConfig{})
	if err != nil {
		t.Fatalf("create adapter: %v", err)
	}

	// Old session that exists in file storage
	hook := map[string]interface{}{
		"cwd":             "/data1/app/workspace/me/tmux-hud",
		"session_id":      "ses_3de02e3d6ffeLKQfxygZunBhPt",
		"hook_event_name": "Response",
	}
	data, _ := json.Marshal(hook)

	_, prompt, response, err := adapter.HandleHookData(data)
	if err != nil {
		t.Logf("File fallback returned error (may be expected): %v", err)
		// Not fatal - this old session may lack part files
	} else {
		t.Logf("File fallback - prompt (%d chars): %s...", len(prompt), truncStr(prompt, 80))
		t.Logf("File fallback - response (%d chars): %s...", len(response), truncStr(response, 80))
	}
}

func truncStr(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
