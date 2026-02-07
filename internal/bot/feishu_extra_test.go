package bot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFeishuBot_SetEncryptKey tests the SetEncryptKey method
func TestFeishuBot_SetEncryptKey(t *testing.T) {
	bot := &FeishuBot{}

	// Test setting encrypt key
	bot.SetEncryptKey("test-encrypt-key")
	assert.Equal(t, "test-encrypt-key", bot.encryptKey)

	// Test updating encrypt key
	bot.SetEncryptKey("new-encrypt-key")
	assert.Equal(t, "new-encrypt-key", bot.encryptKey)

	// Test setting empty key
	bot.SetEncryptKey("")
	assert.Equal(t, "", bot.encryptKey)
}

// TestFeishuBot_SetVerificationToken tests the SetVerificationToken method
func TestFeishuBot_SetVerificationToken(t *testing.T) {
	bot := &FeishuBot{}

	// Test setting verification token
	bot.SetVerificationToken("test-token")
	assert.Equal(t, "test-token", bot.verificationToken)

	// Test updating verification token
	bot.SetVerificationToken("new-token")
	assert.Equal(t, "new-token", bot.verificationToken)

	// Test setting empty token
	bot.SetVerificationToken("")
	assert.Equal(t, "", bot.verificationToken)
}

// TestFeishuBot_SetMessageHandler tests the SetMessageHandler method
func TestFeishuBot_SetMessageHandler(t *testing.T) {
	bot := &FeishuBot{}

	// Verify initial handler is nil
	assert.Nil(t, bot.GetMessageHandler())

	// Test setting message handler
	called := false
	handler := func(msg BotMessage) {
		called = true
	}
	bot.SetMessageHandler(handler)

	// Verify handler was set
	retrievedHandler := bot.GetMessageHandler()
	assert.NotNil(t, retrievedHandler)

	// Test that the handler works
	retrievedHandler(BotMessage{})
	assert.True(t, called, "handler should be called")

	// Test updating handler
	newCalled := false
	newHandler := func(msg BotMessage) {
		newCalled = true
	}
	bot.SetMessageHandler(newHandler)
	bot.GetMessageHandler()(BotMessage{})
	assert.True(t, newCalled, "new handler should be called")
}

// TestFeishuBot_GetMessageHandler tests the GetMessageHandler method
func TestFeishuBot_GetMessageHandler(t *testing.T) {
	bot := &FeishuBot{}

	// Test getting handler when none is set
	assert.Nil(t, bot.GetMessageHandler())

	// Test getting handler after setting one
	handler := func(msg BotMessage) {
		// Test handler
	}
	bot.SetMessageHandler(handler)
	assert.NotNil(t, bot.GetMessageHandler())
}

// TestEscapeJSONString_AdditionalCases adds more test cases
func TestEscapeJSONString_AdditionalCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "with carriage return",
			input:    "line1\rline2",
			expected: "line1\\rline2",
		},
		{
			name:     "with tab",
			input:    "col1\tcol2",
			expected: "col1\\tcol2",
		},
		{
			name:     "mixed special chars",
			input:    "quote: \"\nbackslash: \\\r",
			expected: "quote: \\\"\\nbackslash: \\\\\\r",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "unicode characters",
			input:    "hello 世界",
			expected: "hello 世界",
		},
		{
			name:     "all escape sequences",
			input:    "\"\\\n\r\t",
			expected: "\\\"\\\\\\n\\r\\t",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeJSONString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
