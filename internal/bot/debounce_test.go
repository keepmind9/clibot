package bot

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPendingQueue_AddAndFlush(t *testing.T) {
	var mu sync.Mutex
	var flushed []BotMessage

	pq := NewPendingQueue(func(msgs []BotMessage) {
		mu.Lock()
		flushed = append(flushed, msgs...)
		mu.Unlock()
	})
	defer pq.Stop()

	msg := BotMessage{Content: "hello", Channel: "ch1"}
	pq.Add("ch1", msg, 50*time.Millisecond)

	// Wait for the debounce window to expire
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, flushed, 1)
	assert.Equal(t, "hello", flushed[0].Content)
}

func TestPendingQueue_CoalescesMultipleMessages(t *testing.T) {
	var mu sync.Mutex
	var flushed []BotMessage

	pq := NewPendingQueue(func(msgs []BotMessage) {
		mu.Lock()
		flushed = append(flushed, msgs...)
		mu.Unlock()
	})
	defer pq.Stop()

	pq.Add("ch1", BotMessage{Content: "msg1"}, 100*time.Millisecond)
	pq.Add("ch1", BotMessage{Content: "msg2"}, 100*time.Millisecond)
	pq.Add("ch1", BotMessage{Content: "msg3"}, 100*time.Millisecond)

	// All three should be coalesced into one flush
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, flushed, 3)
	assert.Equal(t, "msg1", flushed[0].Content)
	assert.Equal(t, "msg2", flushed[1].Content)
	assert.Equal(t, "msg3", flushed[2].Content)
}

func TestPendingQueue_BlockUnblock(t *testing.T) {
	var mu sync.Mutex
	var flushed []BotMessage

	pq := NewPendingQueue(func(msgs []BotMessage) {
		mu.Lock()
		flushed = append(flushed, msgs...)
		mu.Unlock()
	})
	defer pq.Stop()

	pq.Add("ch1", BotMessage{Content: "blocked-msg"}, 50*time.Millisecond)

	// Block before the window expires
	time.Sleep(20 * time.Millisecond)
	pq.Block("ch1")

	// Wait well past the original window
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	assert.Len(t, flushed, 0, "no messages should be flushed while blocked")
	mu.Unlock()

	// Unblock starts a fresh timer
	pq.Unblock("ch1")
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	assert.Len(t, flushed, 1, "message should be flushed after unblock")
	assert.Equal(t, "blocked-msg", flushed[0].Content)
	mu.Unlock()
}

func TestPendingQueue_DifferentChannels(t *testing.T) {
	var mu sync.Mutex
	var flushed []BotMessage

	pq := NewPendingQueue(func(msgs []BotMessage) {
		mu.Lock()
		flushed = append(flushed, msgs...)
		mu.Unlock()
	})
	defer pq.Stop()

	pq.Add("ch1", BotMessage{Content: "ch1-msg1"}, 50*time.Millisecond)
	pq.Add("ch2", BotMessage{Content: "ch2-msg1"}, 80*time.Millisecond)
	pq.Add("ch1", BotMessage{Content: "ch1-msg2"}, 50*time.Millisecond)

	// ch1 fires at ~100ms (50ms after last add), ch2 at ~80ms
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, flushed, 3)
	// ch2 should have flushed first (single msg), then ch1 (2 msgs)
	// But order depends on timing, so just check all are present
	contents := make(map[string]bool)
	for _, m := range flushed {
		contents[m.Content] = true
	}
	assert.True(t, contents["ch1-msg1"])
	assert.True(t, contents["ch1-msg2"])
	assert.True(t, contents["ch2-msg1"])
}

func TestPendingQueue_StopDropsPending(t *testing.T) {
	var mu sync.Mutex
	var flushed []BotMessage

	pq := NewPendingQueue(func(msgs []BotMessage) {
		mu.Lock()
		flushed = append(flushed, msgs...)
		mu.Unlock()
	})

	pq.Add("ch1", BotMessage{Content: "will-be-dropped"}, 50*time.Millisecond)
	pq.Stop()

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, flushed, 0, "messages should not be flushed after Stop")
}

func TestPendingQueue_AddAfterStop(t *testing.T) {
	var mu sync.Mutex
	var flushed []BotMessage

	pq := NewPendingQueue(func(msgs []BotMessage) {
		mu.Lock()
		flushed = append(flushed, msgs...)
		mu.Unlock()
	})

	pq.Stop()
	pq.Add("ch1", BotMessage{Content: "after-stop"}, 50*time.Millisecond)

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, flushed, 0, "messages added after Stop should be dropped")
}
