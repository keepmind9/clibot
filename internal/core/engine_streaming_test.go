package core

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/keepmind9/clibot/internal/bot"
	"github.com/keepmind9/clibot/internal/cli"
	"github.com/stretchr/testify/assert"
)

// mockStreamingCLI implements cli.StreamingCLI for testing.
type mockStreamingCLI struct {
	events []cli.CLIEvent
	err    error
	ch     chan cli.CLIEvent
}

func (m *mockStreamingCLI) SendInput(sessionName, input string) error { return nil }
func (m *mockStreamingCLI) HandleHookData(data []byte) (string, string, string, error) {
	return "", "", "", nil
}
func (m *mockStreamingCLI) IsSessionAlive(sessionName string) bool { return true }
func (m *mockStreamingCLI) CreateSession(sessionName string, opts ...cli.SessionOption) error {
	return nil
}
func (m *mockStreamingCLI) StopSession(sessionName string) error { return nil }

func (m *mockStreamingCLI) SendInputStreaming(sessionName, input string) (<-chan cli.CLIEvent, error) {
	if m.err != nil {
		return nil, m.err
	}
	ch := make(chan cli.CLIEvent, len(m.events)+1)
	for _, evt := range m.events {
		ch <- evt
	}
	ch <- cli.CLIEvent{Type: cli.CLIEventDone}
	close(ch)
	m.ch = ch
	return ch, nil
}

// mockRichMessenger implements bot.RichMessenger for testing.
type mockRichMessenger struct {
	handle    *mockRichHandle
	createErr error
	mu        sync.Mutex
}

func (m *mockRichMessenger) CreateRichMessage(channel string, opts bot.RichMessageOptions) (bot.RichMessageHandle, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	h := &mockRichHandle{channel: channel, blocks: make(map[int][]bot.ContentBlock)}
	m.mu.Lock()
	m.handle = h
	m.mu.Unlock()
	return h, nil
}

func (m *mockRichMessenger) getHandle() *mockRichHandle {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.handle
}

type mockRichHandle struct {
	channel    string
	blocks     map[int][]bot.ContentBlock
	updateCnt  int
	finishCnt  int
	finishCall int
	finished   atomic.Bool
}

func (h *mockRichHandle) Channel() string { return h.channel }

func (h *mockRichHandle) Update(blocks []bot.ContentBlock) error {
	h.updateCnt++
	h.blocks[h.updateCnt] = append([]bot.ContentBlock{}, blocks...)
	return nil
}

func (h *mockRichHandle) Finish(blocks []bot.ContentBlock) error {
	h.finishCnt++
	h.finishCall++
	h.blocks[-h.finishCall] = append([]bot.ContentBlock{}, blocks...)
	h.finished.Store(true)
	return nil
}

// mockRichMessengerBot combines BotAdapter + RichMessenger for testing.
type mockRichBot struct {
	mockBotAdapter
	rich *mockRichMessenger
}

func (m *mockRichBot) CreateRichMessage(channel string, opts bot.RichMessageOptions) (bot.RichMessageHandle, error) {
	return m.rich.CreateRichMessage(channel, opts)
}

func newStreamingTestEngine() (*Engine, *mockStreamingCLI, *mockRichBot) {
	config := &Config{
		Sessions: []SessionConfig{{Name: "default", CLIType: "test-stream", WorkDir: "/tmp"}},
		Bots: map[string]BotConfig{
			"testbot": {Enabled: true, ReplyMode: "card"},
		},
		Security: SecurityConfig{
			WhitelistEnabled: false,
		},
	}
	engine := NewEngine(config)

	streamCLI := &mockStreamingCLI{}
	richBot := &mockRichBot{rich: &mockRichMessenger{}}

	engine.RegisterCLIAdapter("test-stream", streamCLI)
	engine.RegisterBotAdapter("testbot", richBot)

	return engine, streamCLI, richBot
}

func TestHandleStreamingReply_HappyPath(t *testing.T) {
	engine, streamCLI, richBot := newStreamingTestEngine()

	streamCLI.events = []cli.CLIEvent{
		{Type: cli.CLIEventText, Content: "Hello "},
		{Type: cli.CLIEventText, Content: "world"},
		{Type: cli.CLIEventToolUse, ToolName: "Bash", ToolMeta: map[string]string{"command": "ls"}},
		{Type: cli.CLIEventDone, Content: "Done!"},
	}

	session := &Session{Name: "default", State: StateIdle}
	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "ch1",
		UserID:   "admin1",
	}

	engine.handleStreamingReply(streamCLI, richBot.rich, session, msg, "test input", "ch1")

	assert.NotNil(t, richBot.rich.handle)
	assert.True(t, richBot.rich.handle.finished.Load())
	assert.Equal(t, 1, richBot.rich.handle.finishCnt)

	// Verify the final blocks contain text and tool call
	finalBlocks := richBot.rich.handle.blocks[-1]
	assert.True(t, len(finalBlocks) > 0)
}

func TestHandleStreamingReply_CardCreateFails(t *testing.T) {
	engine, streamCLI, richBot := newStreamingTestEngine()

	richBot.rich.createErr = assert.AnError

	session := &Session{Name: "default", State: StateIdle}
	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "ch1",
		UserID:   "admin1",
	}

	// Should not panic, falls back to text
	engine.handleStreamingReply(streamCLI, richBot.rich, session, msg, "test input", "ch1")
	assert.Nil(t, richBot.rich.handle)
}

func TestHandleStreamingReply_StreamStartFails(t *testing.T) {
	engine, streamCLI, richBot := newStreamingTestEngine()

	streamCLI.err = assert.AnError

	session := &Session{Name: "default", State: StateIdle}
	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "ch1",
		UserID:   "admin1",
	}

	engine.handleStreamingReply(streamCLI, richBot.rich, session, msg, "test input", "ch1")

	// Should finish card with error message
	assert.NotNil(t, richBot.rich.handle)
	assert.True(t, richBot.rich.handle.finished.Load())
}

func TestHandleStreamingReply_EmptyEvents(t *testing.T) {
	engine, streamCLI, richBot := newStreamingTestEngine()

	streamCLI.events = []cli.CLIEvent{}

	session := &Session{Name: "default", State: StateIdle}
	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "ch1",
		UserID:   "admin1",
	}

	engine.handleStreamingReply(streamCLI, richBot.rich, session, msg, "test input", "ch1")

	assert.NotNil(t, richBot.rich.handle)
	assert.True(t, richBot.rich.handle.finished.Load())
}

func TestHandleStreamingReply_Throttle(t *testing.T) {
	engine, streamCLI, richBot := newStreamingTestEngine()

	// Generate many events to test throttling
	var events []cli.CLIEvent
	for i := 0; i < 20; i++ {
		events = append(events, cli.CLIEvent{Type: cli.CLIEventText, Content: "chunk"})
	}
	streamCLI.events = events

	session := &Session{Name: "default", State: StateIdle}
	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "ch1",
		UserID:   "admin1",
	}

	start := time.Now()
	engine.handleStreamingReply(streamCLI, richBot.rich, session, msg, "test input", "ch1")
	elapsed := time.Since(start)

	_ = elapsed
	assert.NotNil(t, richBot.rich.handle)
	assert.True(t, richBot.rich.handle.finished.Load())
	// With 500ms throttle and 20 events sent instantly via buffered channel,
	// updates should be limited (not 20 updates)
	assert.Less(t, richBot.rich.handle.updateCnt, 20)
}

func TestStreamingReplyMode_TextMode(t *testing.T) {
	// When reply_mode is "text", streaming path should be skipped
	config := &Config{
		Sessions: []SessionConfig{{Name: "default", CLIType: "test-stream", WorkDir: "/tmp"}},
		Bots: map[string]BotConfig{
			"testbot": {Enabled: true, ReplyMode: "text"},
		},
		Security: SecurityConfig{
			WhitelistEnabled: false,
		},
	}
	engine := NewEngine(config)

	streamCLI := &mockStreamingCLI{}
	richBot := &mockRichBot{rich: &mockRichMessenger{}}

	engine.RegisterCLIAdapter("test-stream", streamCLI)
	engine.RegisterBotAdapter("testbot", richBot)
	engine.initializeSessions()

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "ch1",
		UserID:   "admin1",
		Content:  "hello",
	}

	// This should go through normal SendInput path, not streaming
	engine.HandleUserMessage(msg)

	// Card should not be created
	assert.Nil(t, richBot.rich.handle)
}

func TestHandleStreamingReply_ThinkingAndToolResult(t *testing.T) {
	engine, streamCLI, richBot := newStreamingTestEngine()

	streamCLI.events = []cli.CLIEvent{
		{Type: cli.CLIEventThinking, Content: "Let me think..."},
		{Type: cli.CLIEventText, Content: "Here's the answer"},
		{Type: cli.CLIEventToolUse, ToolName: "Read", ToolMeta: map[string]string{"file_path": "/tmp/test.go"}},
		{Type: cli.CLIEventToolResult, Content: "file contents"},
		{Type: cli.CLIEventDone},
	}

	session := &Session{Name: "default", State: StateIdle}
	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "ch1",
		UserID:   "admin1",
	}

	engine.handleStreamingReply(streamCLI, richBot.rich, session, msg, "test input", "ch1")

	assert.NotNil(t, richBot.rich.handle)
	assert.True(t, richBot.rich.handle.finished.Load())

	finalBlocks := richBot.rich.handle.blocks[-1]
	// Should have: thinking, text, tool_call, tool_result
	assert.Len(t, finalBlocks, 4)
	assert.Equal(t, bot.ContentBlockThinking, finalBlocks[0].Type)
	assert.Equal(t, bot.ContentBlockText, finalBlocks[1].Type)
	assert.Equal(t, bot.ContentBlockToolCall, finalBlocks[2].Type)
	assert.Equal(t, bot.ContentBlockToolResult, finalBlocks[3].Type)
}

func TestHandleStreamingReply_CancelContext(t *testing.T) {
	engine, _, richBot := newStreamingTestEngine()
	ctx, cancel := context.WithCancel(context.Background())
	engine.ctx = ctx
	engine.cancel = cancel

	// Use a custom StreamingCLI that blocks until context cancelled
	streamCLI := &cancelMockCLI{ctx: ctx}

	session := &Session{Name: "default", State: StateIdle}
	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "ch1",
		UserID:   "admin1",
	}

	done := make(chan struct{})
	go func() {
		engine.handleStreamingReply(streamCLI, richBot.rich, session, msg, "test input", "ch1")
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
		assert.NotNil(t, richBot.rich.handle)
	case <-time.After(2 * time.Second):
		t.Fatal("handleStreamingReply did not complete after context cancel")
	}
}

type cancelMockCLI struct {
	ctx context.Context
}

func (m *cancelMockCLI) SendInput(sessionName, input string) error { return nil }
func (m *cancelMockCLI) HandleHookData(data []byte) (string, string, string, error) {
	return "", "", "", nil
}
func (m *cancelMockCLI) IsSessionAlive(sessionName string) bool { return true }
func (m *cancelMockCLI) CreateSession(sessionName string, opts ...cli.SessionOption) error {
	return nil
}
func (m *cancelMockCLI) StopSession(sessionName string) error { return nil }

func (m *cancelMockCLI) SendInputStreaming(sessionName, input string) (<-chan cli.CLIEvent, error) {
	ch := make(chan cli.CLIEvent)
	go func() {
		ch <- cli.CLIEvent{Type: cli.CLIEventText, Content: "partial"}
		<-m.ctx.Done()
		close(ch)
	}()
	return ch, nil
}
