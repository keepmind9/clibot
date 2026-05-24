package bot

import (
	"sync"
	"time"

	"github.com/keepmind9/clibot/internal/logger"
)

// PendingQueue coalesces messages per key using configurable debounce windows.
// When a message is added, the timer resets. When the window expires, all
// accumulated messages for that key are flushed to the handler in a single batch.
type PendingQueue struct {
	mu      sync.Mutex
	handler func([]BotMessage)
	pending map[string]*pendingEntry
	stopped bool
}

type pendingEntry struct {
	messages []BotMessage
	timer    *time.Timer
	window   time.Duration
}

// NewPendingQueue creates a new PendingQueue that batches messages per key.
func NewPendingQueue(handler func([]BotMessage)) *PendingQueue {
	return &PendingQueue{
		handler: handler,
		pending: make(map[string]*pendingEntry),
	}
}

// Add adds a message to the pending queue for the given key and resets the debounce timer.
// If the queue is stopped, the message is dropped.
func (pq *PendingQueue) Add(key string, msg BotMessage, window time.Duration) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.stopped {
		logger.WithField("key", key).Warn("pending-queue-add-after-stop: message dropped")
		return
	}

	entry, exists := pq.pending[key]
	if !exists {
		entry = &pendingEntry{
			window: window,
		}
		pq.pending[key] = entry
	}

	entry.messages = append(entry.messages, msg)

	if entry.timer != nil {
		entry.timer.Stop()
	}
	entry.window = window
	entry.timer = time.AfterFunc(window, func() {
		pq.flush(key)
	})
}

// Block stops the timer for the given key but keeps accumulated messages.
// Useful for holding messages while processing.
func (pq *PendingQueue) Block(key string) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	entry, exists := pq.pending[key]
	if !exists {
		return
	}

	if entry.timer != nil {
		entry.timer.Stop()
		entry.timer = nil
	}
}

// Unblock restarts the debounce timer for the given key using the stored window duration.
// If there are no pending messages, this is a no-op.
func (pq *PendingQueue) Unblock(key string) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	entry, exists := pq.pending[key]
	if !exists || len(entry.messages) == 0 {
		return
	}

	if entry.timer != nil {
		entry.timer.Stop()
	}
	entry.timer = time.AfterFunc(entry.window, func() {
		pq.flush(key)
	})
}

// Stop cleans up all pending timers. No further flushes will occur.
func (pq *PendingQueue) Stop() {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	pq.stopped = true
	for _, entry := range pq.pending {
		if entry.timer != nil {
			entry.timer.Stop()
		}
	}
	pq.pending = make(map[string]*pendingEntry)
}

// flush delivers all accumulated messages for a key to the handler.
func (pq *PendingQueue) flush(key string) {
	pq.mu.Lock()

	entry, exists := pq.pending[key]
	if !exists {
		pq.mu.Unlock()
		return
	}

	messages := entry.messages
	delete(pq.pending, key)
	pq.mu.Unlock()

	if len(messages) > 0 && pq.handler != nil {
		pq.handler(messages)
	}
}
