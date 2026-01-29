package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Message represents a single message in a conversation
type Message struct {
	Role      string    `json:"role"`      // "user" or "assistant"
	Content   string    `json:"content"`   // Message content
	Timestamp time.Time `json:"timestamp"` // Message timestamp
}

// Conversation represents a Claude Code conversation file
type Conversation struct {
	Messages []Message `json:"messages"`
}

// LastAssistantMessage returns the last message from the assistant
func (c *Conversation) LastAssistantMessage() *Message {
	if len(c.Messages) == 0 {
		return nil
	}

	// Iterate backwards to find the last assistant message
	for i := len(c.Messages) - 1; i >= 0; i-- {
		if c.Messages[i].Role == "assistant" {
			return &c.Messages[i]
		}
	}

	return nil
}

// LoadConversation loads a conversation from a JSON file
func LoadConversation(filePath string) (*Conversation, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var conv Conversation
	if err := json.Unmarshal(data, &conv); err != nil {
		return nil, err
	}

	return &conv, nil
}

// FindLatestConversationFile finds the latest conversation file in a directory
func FindLatestConversationFile(dir string) (string, error) {
	// Find all JSON files in the directory
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return "", err
	}

	if len(files) == 0 {
		return "", os.ErrNotExist
	}

	// Sort files by modification time (newest first)
	sort.Slice(files, func(i, j int) bool {
		infoI, _ := os.Stat(files[i])
		infoJ, _ := os.Stat(files[j])
		return infoI.ModTime().After(infoJ.ModTime())
	})

	return files[0], nil
}

// GetLastAssistantContent extracts the last assistant message content from the latest conversation
func GetLastAssistantContent(dir string) (string, error) {
	// Find latest conversation file
	latestFile, err := FindLatestConversationFile(dir)
	if err != nil {
		return "", err
	}

	// Load conversation
	conv, err := LoadConversation(latestFile)
	if err != nil {
		return "", err
	}

	// Get last assistant message
	msg := conv.LastAssistantMessage()
	if msg == nil {
		return "", os.ErrInvalid
	}

	return msg.Content, nil
}
