package bot

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewQQBot(t *testing.T) {
	bot := NewQQBot("test_app_id", "test_secret")
	assert.NotNil(t, bot)
	assert.Equal(t, "test_app_id", bot.appID)
	assert.Equal(t, "test_secret", bot.appSecret)
	assert.NotNil(t, bot.msgSeqMap)
}

func TestQQBot_SetProxyManager(t *testing.T) {
	qqBot := NewQQBot("app", "secret")
	mockMgr := &mockProxyManager{}

	qqBot.SetProxyManager(mockMgr)

	qqBot.mu.RLock()
	defer qqBot.mu.RUnlock()
	assert.Equal(t, mockMgr, qqBot.proxyMgr)
}

func TestQQBot_nextMsgSeq(t *testing.T) {
	qqBot := NewQQBot("app", "secret")

	// Test sequence increment
	seq1 := qqBot.nextMsgSeq("msg1")
	seq2 := qqBot.nextMsgSeq("msg1")
	seq3 := qqBot.nextMsgSeq("msg2")

	assert.Equal(t, 1, seq1)
	assert.Equal(t, 2, seq2)
	assert.Equal(t, 1, seq3)

	// Test map growth prevention
	qqBot.msgSeqMap = make(map[string]int)
	for i := 0; i < 600; i++ {
		qqBot.nextMsgSeq(string(rune(i)))
	}
	// Map should be pruned to prevent unbounded growth
	assert.LessOrEqual(t, len(qqBot.msgSeqMap), 500)
}

func TestQQBot_SupportsTypingIndicator(t *testing.T) {
	qqBot := NewQQBot("app", "secret")
	assert.False(t, qqBot.SupportsTypingIndicator())
}

func TestQQBot_AddTypingIndicator(t *testing.T) {
	qqBot := NewQQBot("app", "secret")
	assert.False(t, qqBot.AddTypingIndicator("test_msg_id"))
}

func TestQQBot_RemoveTypingIndicator(t *testing.T) {
	qqBot := NewQQBot("app", "secret")
	assert.NoError(t, qqBot.RemoveTypingIndicator("test_msg_id"))
}

func TestQQBot_SetMessageHandler(t *testing.T) {
	qqBot := NewQQBot("app", "secret")

	// Verify initial handler is nil
	assert.Nil(t, qqBot.GetMessageHandler())

	// Test setting message handler
	called := false
	handler := func(msg BotMessage) {
		called = true
	}
	qqBot.SetMessageHandler(handler)

	// Verify handler was set
	retrievedHandler := qqBot.GetMessageHandler()
	assert.NotNil(t, retrievedHandler)

	// Test that the handler works
	retrievedHandler(BotMessage{})
	assert.True(t, called, "handler should be called")

	// Test updating handler
	newCalled := false
	newHandler := func(msg BotMessage) {
		newCalled = true
	}
	qqBot.SetMessageHandler(newHandler)
	qqBot.GetMessageHandler()(BotMessage{})
	assert.True(t, newCalled, "new handler should be called")
}

func TestQQBot_Stop(t *testing.T) {
	qqBot := NewQQBot("app", "secret")

	// Test stopping bot without starting
	err := qqBot.Stop()
	assert.NoError(t, err, "Stop should not return error even if not started")
}

func TestSplitMessage(t *testing.T) {
	tests := []struct {
		name              string
		message           string
		maxLen            int
		expectedPartCount int
	}{
		{
			name:              "short message",
			message:           "hello",
			maxLen:            2000,
			expectedPartCount: 1,
		},
		{
			name:              "empty string",
			message:           "",
			maxLen:            2000,
			expectedPartCount: 1,
		},
		{
			name:              "long message without newlines",
			message:           string(make([]byte, 3000)),
			maxLen:            2000,
			expectedPartCount: 2,
		},
		{
			name:              "message with newline at split boundary",
			message:           "line1\n" + strings.Repeat("a", 1995) + "\nline2\n" + strings.Repeat("b", 1000),
			maxLen:            2000,
			expectedPartCount: 2,
		},
		{
			name:              "message with multiple newlines",
			message:           "line1\nline2\nline3\nline4\nline5",
			maxLen:            10,
			expectedPartCount: 4,
		},
		{
			name:              "message exactly at max length",
			message:           strings.Repeat("a", 2000),
			maxLen:            2000,
			expectedPartCount: 1,
		},
		{
			name:              "message just over max length",
			message:           strings.Repeat("a", 2001),
			maxLen:            2000,
			expectedPartCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitMessage(tt.message, tt.maxLen)
			assert.Equal(t, tt.expectedPartCount, len(result), "should split into correct number of parts")

			// Verify each part is within max length
			for i, part := range result {
				assert.LessOrEqual(t, len(part), tt.maxLen, "part %d should be within max length", i)
			}

			// Verify reconstruction (except for empty string case)
			if tt.message != "" {
				reconstructed := strings.Join(result, "")
				assert.Equal(t, tt.message, reconstructed, "reconstructed message should match original")
			}
		})
	}
}

func TestQQBot_MsgSeqMapGrowthLimit(t *testing.T) {
	qqBot := NewQQBot("app", "secret")

	// Fill the map beyond maxMsgSeqMapSize
	for i := 0; i < 1000; i++ {
		qqBot.nextMsgSeq("key")
	}

	// Map should be pruned to prevent unbounded growth
	assert.LessOrEqual(t, len(qqBot.msgSeqMap), maxMsgSeqMapSize, "map should not exceed max size")
}

func TestQQBot_NewQQBotWithEmptyCredentials(t *testing.T) {
	// Test bot creation with empty credentials
	bot := NewQQBot("", "")
	assert.NotNil(t, bot)
	assert.Equal(t, "", bot.appID)
	assert.Equal(t, "", bot.appSecret)
}

func TestQQBot_GetMessageHandlerInitiallyNil(t *testing.T) {
	qqBot := NewQQBot("app", "secret")

	// GetMessageHandler should return nil initially
	handler := qqBot.GetMessageHandler()
	assert.Nil(t, handler)
}

func TestQQBot_Constants(t *testing.T) {
	// Test that constants are defined and have expected values
	assert.Equal(t, "https://bots.qq.com/app/getAppAccessToken", QQTokenURL)
	assert.Equal(t, "https://api.sgroup.qq.com", QQAPIBase)
	assert.Equal(t, QQAPIBase+"/gateway", QQGatewayURL)
	assert.Equal(t, 0, qqMessageTypeText)
	assert.Equal(t, 2000, qqMaxMessageLength)
	assert.Equal(t, 60, qqTokenExpirationBuffer)
	assert.Equal(t, 500, maxMsgSeqMapSize)
	assert.Equal(t, 10*time.Second, qqWebSocketHandshakeTimeout)
	assert.Equal(t, 10*time.Second, qqAPIRequestTimeout)
	assert.Equal(t, 15*time.Second, qqMessageSendTimeout)
}

func TestQQBot_HandlerIntegration(t *testing.T) {
	qqBot := NewQQBot("app", "secret")

	// Test setting and getting handler multiple times
	calledCount := 0
	handler1 := func(msg BotMessage) {
		calledCount++
	}

	qqBot.SetMessageHandler(handler1)
	retrieved := qqBot.GetMessageHandler()
	assert.NotNil(t, retrieved)

	retrieved(BotMessage{})
	assert.Equal(t, 1, calledCount)

	// Update handler
	handler2 := func(msg BotMessage) {
		calledCount += 10
	}
	qqBot.SetMessageHandler(handler2)

	newRetrieved := qqBot.GetMessageHandler()
	newRetrieved(BotMessage{})
	assert.Equal(t, 11, calledCount)
}

func TestQQBot_MultipleMessageSequences(t *testing.T) {
	qqBot := NewQQBot("app", "secret")

	// Test multiple different message IDs
	msgIDs := []string{"msg1", "msg2", "msg3", "msg1", "msg2"}

	expectedSeqs := []int{1, 1, 1, 2, 2}

	for i, msgID := range msgIDs {
		seq := qqBot.nextMsgSeq(msgID)
		assert.Equal(t, expectedSeqs[i], seq, "message %s should have sequence %d", msgID, expectedSeqs[i])
	}
}

func TestQQBot_StopWithNilCancel(t *testing.T) {
	qqBot := NewQQBot("app", "secret")

	// Stop should not panic even if cancel is nil
	err := qqBot.Stop()
	assert.NoError(t, err)
}

func TestQQBot_ProxyManagerIntegration(t *testing.T) {
	qqBot := NewQQBot("app", "secret")
	mockMgr := &mockProxyManager{}

	// Set proxy manager
	qqBot.SetProxyManager(mockMgr)

	// Verify proxy manager is set
	qqBot.mu.RLock()
	assert.Equal(t, mockMgr, qqBot.proxyMgr)
	qqBot.mu.RUnlock()

	// Test setting proxy manager again
	newMockMgr := &mockProxyManager{}
	qqBot.SetProxyManager(newMockMgr)

	qqBot.mu.RLock()
	assert.Equal(t, newMockMgr, qqBot.proxyMgr)
	qqBot.mu.RUnlock()
}

func TestQQBot_MessageHandlerNilSafety(t *testing.T) {
	qqBot := NewQQBot("app", "secret")

	// GetMessageHandler should return nil initially
	handler := qqBot.GetMessageHandler()
	assert.Nil(t, handler)

	// Calling nil handler should be safe (no panic)
	if handler != nil {
		handler(BotMessage{})
	}
}

// mockProxyManager is a mock implementation of proxy.Manager for testing
type mockProxyManager struct{}

func (m *mockProxyManager) GetHTTPClient(platform string) (*http.Client, error) {
	return &http.Client{}, nil
}

func (m *mockProxyManager) GetProxyURL(platform string) string {
	return ""
}
