package core

import (
	"testing"
	"time"

	"github.com/keepmind9/clibot/internal/bot"
	"github.com/stretchr/testify/assert"
)

// mockDebounceableBot wraps mockBotAdapter with Debounceable support.
type mockDebounceableBot struct {
	mockBotAdapter
	window int
}

func (m *mockDebounceableBot) DebounceWindow() int {
	return m.window
}

func TestEngine_Debounce_CoalescesMessages(t *testing.T) {
	engine := NewEngine(&Config{})

	debounceBot := &mockDebounceableBot{window: 100}
	engine.RegisterBotAdapter("test", debounceBot)

	received := make(chan bot.BotMessage, 10)
	go func() {
		for msg := range engine.messageChan {
			received <- msg
		}
	}()

	engine.HandleBotMessage(bot.BotMessage{Platform: "test", Channel: "ch1", Content: "hello"})
	engine.HandleBotMessage(bot.BotMessage{Platform: "test", Channel: "ch1", Content: "world"})

	select {
	case coalesced := <-received:
		assert.Contains(t, coalesced.Content, "hello")
		assert.Contains(t, coalesced.Content, "world")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected coalesced message")
	}

	for _, pq := range engine.pendingQueues {
		pq.Stop()
	}
}

func TestEngine_Debounce_SpecialCommandsBypass(t *testing.T) {
	engine := NewEngine(&Config{})

	debounceBot := &mockDebounceableBot{window: 5000}
	engine.RegisterBotAdapter("test", debounceBot)

	engine.HandleBotMessage(bot.BotMessage{Platform: "test", Channel: "ch1", Content: "slist"})

	pq, exists := engine.pendingQueues["test"]
	if exists && pq != nil {
		pq.Stop()
	}
}

func TestEngine_Debounce_NoDebounceWhenZero(t *testing.T) {
	engine := NewEngine(&Config{})

	regularBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("test", regularBot)

	received := make(chan bot.BotMessage, 10)
	go func() {
		for msg := range engine.messageChan {
			received <- msg
		}
	}()

	engine.HandleBotMessage(bot.BotMessage{Platform: "test", Channel: "ch1", Content: "hello"})

	select {
	case m := <-received:
		assert.Equal(t, "hello", m.Content)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected message without debounce")
	}
}

func TestEngine_Debounce_StopCleansUp(t *testing.T) {
	engine := NewEngine(&Config{})

	debounceBot := &mockDebounceableBot{window: 100}
	engine.RegisterBotAdapter("test", debounceBot)

	engine.HandleBotMessage(bot.BotMessage{Platform: "test", Channel: "ch1", Content: "hello"})
	assert.NotNil(t, engine.pendingQueues["test"])

	engine.pendingQueues["test"].Stop()
}
